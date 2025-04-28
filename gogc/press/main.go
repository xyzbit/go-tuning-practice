package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"
)

var (
	host     = flag.String("host", "localhost", "服务器主机名或IP")
	port     = flag.Int("port", 8080, "服务器端口")
	duration = flag.Int("duration", 0, "测试持续时间(秒)，0表示永久运行")
	rps      = flag.Int("rps", 100, "基础每秒请求数")
	workers  = flag.Int("workers", 10, "并发工作协程数")
	loadType = flag.String("load-type", "constant", "负载类型: constant(固定), wave(波动), spike(尖刺)")
)

// 要请求的端点
var endpoints = []string{
	"/",
}

// 统计数据
var (
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	totalLatency       int64
	maxLatency         int64
	minLatency         int64 = int64(time.Hour)
)

// 控制信号
var (
	stop = make(chan os.Signal, 1)
)

func main() {
	flag.Parse()
	signal.Notify(stop, os.Interrupt)

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 负载控制通道
	loadControl := make(chan struct{}, *rps)

	// 启动负载控制器
	switch *loadType {
	case "constant":
		go constantLoadController(loadControl)
	case "wave":
		go waveLoadController(loadControl)
	case "spike":
		go spikeLoadController(loadControl)
	default:
		log.Printf("未知的负载类型: %s, 使用默认的固定负载", *loadType)
		go constantLoadController(loadControl)
	}

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-loadControl:
					// 随机选择一个端点
					endpoint := endpoints[rand.Intn(len(endpoints))]
					url := fmt.Sprintf("http://%s:%d%s", *host, *port, endpoint)

					start := time.Now()
					resp, err := client.Get(url)
					elapsed := time.Since(start)

					atomic.AddInt64(&totalRequests, 1)

					if err != nil {
						atomic.AddInt64(&failedRequests, 1)
					} else {
						atomic.AddInt64(&successfulRequests, 1)
						atomic.AddInt64(&totalLatency, elapsed.Nanoseconds())

						// 更新最大延迟
						for {
							old := atomic.LoadInt64(&maxLatency)
							if elapsed.Nanoseconds() <= old || atomic.CompareAndSwapInt64(&maxLatency, old, elapsed.Nanoseconds()) {
								break
							}
						}

						// 更新最小延迟
						for {
							old := atomic.LoadInt64(&minLatency)
							if elapsed.Nanoseconds() >= old || atomic.CompareAndSwapInt64(&minLatency, old, elapsed.Nanoseconds()) {
								break
							}
						}

						resp.Body.Close()
					}
				case <-stop:
					return
				}
			}
		}(i)
	}

	// 启动统计输出协程
	go statsReporter()

	// 等待持续时间或用户中断
	if *duration > 0 {
		time.Sleep(time.Duration(*duration) * time.Second)
		close(stop)
	} else {
		<-stop
	}

	// 等待所有工作协程完成
	wg.Wait()
	printFinalStats()
}

// 固定负载控制器
func constantLoadController(loadControl chan<- struct{}) {
	log.Println("启动固定负载模式")
	interval := time.Second / time.Duration(*rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case loadControl <- struct{}{}:
				// 成功发送请求信号
			default:
				// 通道已满，表示系统处理不过来
				atomic.AddInt64(&failedRequests, 1)
			}
		case <-stop:
			return
		}
	}
}

// 波动负载控制器
func waveLoadController(loadControl chan<- struct{}) {
	log.Println("启动波动负载模式")
	baseTicker := time.NewTicker(time.Millisecond)
	defer baseTicker.Stop()

	cycleTime := 30 * time.Second // 完整波动周期为30秒
	baseRate := float64(*rps)

	var requestsThisSecond int64
	lastResetTime := time.Now()

	for {
		select {
		case <-baseTicker.C:
			now := time.Now()

			// 每秒重置计数器
			if now.Sub(lastResetTime) >= time.Second {
				requestsThisSecond = 0
				lastResetTime = now
			}

			// 计算当前周期中的位置(0-1)
			cycle := float64(now.UnixNano()/1e6) / float64(cycleTime.Milliseconds())
			position := cycle - float64(int(cycle))

			// 正弦波调整：在基础RPS的50%-150%之间波动
			currentRPS := baseRate * (0.5 + 0.5*math.Sin(position*2*math.Pi))

			// 检查是否应该发送请求
			if float64(requestsThisSecond) < currentRPS/float64(1000) {
				select {
				case loadControl <- struct{}{}:
					atomic.AddInt64(&requestsThisSecond, 1)
				default:
					// 通道已满，跳过
				}
			}

		case <-stop:
			return
		}
	}
}

// 尖刺负载控制器
func spikeLoadController(loadControl chan<- struct{}) {
	log.Println("启动尖刺负载模式")
	baseTicker := time.NewTicker(time.Millisecond)
	defer baseTicker.Stop()

	baseRate := float64(*rps)
	spikeRate := baseRate * 5 // 尖刺时请求速率是基础的5倍
	isSpike := false
	nextSpikeTime := time.Now().Add(10 * time.Second)
	var spikeEndTime time.Time

	var requestsThisSecond int64
	lastResetTime := time.Now()

	for {
		select {
		case <-baseTicker.C:
			now := time.Now()

			// 每秒重置计数器
			if now.Sub(lastResetTime) >= time.Second {
				requestsThisSecond = 0
				lastResetTime = now
			}

			// 检查是否处于尖刺状态
			if !isSpike && now.After(nextSpikeTime) {
				// 开始一个尖刺
				isSpike = true
				spikeDuration := time.Duration(2+rand.Intn(3)) * time.Second
				spikeEndTime = now.Add(spikeDuration)
				log.Printf("触发负载尖刺! 持续 %v", spikeDuration)
			} else if isSpike && now.After(spikeEndTime) {
				// 结束尖刺状态
				isSpike = false
				// 下一次尖刺在10-30秒后
				nextSpikeTime = now.Add(time.Duration(10+rand.Intn(20)) * time.Second)
				log.Printf("尖刺结束，下一次尖刺在 %v 秒后", nextSpikeTime.Sub(now).Seconds())
			}

			// 根据当前状态确定请求速率
			currentRPS := baseRate
			if isSpike {
				currentRPS = spikeRate
			}

			// 检查是否应该发送请求
			if float64(requestsThisSecond) < currentRPS/float64(1000) {
				select {
				case loadControl <- struct{}{}:
					atomic.AddInt64(&requestsThisSecond, 1)
				default:
					// 通道已满，跳过
				}
			}

		case <-stop:
			return
		}
	}
}

// 统计报告
func statsReporter() {
	lastTotal := int64(0)
	lastTime := time.Now()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			elapsed := now.Sub(lastTime)
			current := atomic.LoadInt64(&totalRequests)
			successful := atomic.LoadInt64(&successfulRequests)
			failed := atomic.LoadInt64(&failedRequests)
			latency := atomic.LoadInt64(&totalLatency)

			// 计算当前 RPS
			currentRPS := float64(current-lastTotal) / elapsed.Seconds()

			// 计算平均延迟 (如果有成功的请求)
			var avgLatency float64
			if successful > 0 {
				avgLatency = float64(latency) / float64(successful) / 1000000 // 转换为毫秒
			}

			fmt.Printf("[%s] 负载类型: %s, RPS: %.1f, 成功率: %.1f%%, 平均延迟: %.1fms, 总请求: %d (成功: %d, 失败: %d)\n",
				now.Format("15:04:05"),
				*loadType,
				currentRPS,
				float64(successful)/float64(current)*100,
				avgLatency,
				current,
				successful,
				failed)

			lastTotal = current
			lastTime = now
		case <-stop:
			return
		}
	}
}

// 最终统计
func printFinalStats() {
	total := atomic.LoadInt64(&totalRequests)
	successful := atomic.LoadInt64(&successfulRequests)
	failed := atomic.LoadInt64(&failedRequests)

	if total == 0 {
		fmt.Println("未发送任何请求")
		return
	}

	avgLatency := float64(0)
	if successful > 0 {
		avgLatency = float64(atomic.LoadInt64(&totalLatency)) / float64(successful) / 1000000
	}

	min := float64(atomic.LoadInt64(&minLatency)) / 1000000
	max := float64(atomic.LoadInt64(&maxLatency)) / 1000000

	if min == float64(time.Hour)/1000000 {
		min = 0
	}

	fmt.Println("\n---------- 测试结果 ----------")
	fmt.Printf("负载类型: %s\n", *loadType)
	fmt.Printf("总请求数: %d\n", total)
	fmt.Printf("成功请求: %d (%.1f%%)\n", successful, float64(successful)/float64(total)*100)
	fmt.Printf("失败请求: %d (%.1f%%)\n", failed, float64(failed)/float64(total)*100)
	fmt.Printf("平均延迟: %.2fms\n", avgLatency)
	fmt.Printf("最小延迟: %.2fms\n", min)
	fmt.Printf("最大延迟: %.2fms\n", max)
	fmt.Println("-------------------------------")
}
