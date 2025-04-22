package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	_ "net/http/pprof" // 导入 pprof，它会自动注册 HTTP 处理程序
	"runtime"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// 定义 prometheus 指标
var allocObjects = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "alloc_objects_total",
		Help: "分配对象总数",
	},
)

func init() {
	// 注册指标到 prometheus
	prometheus.MustRegister(allocObjects)
}

// 一个占用内存并迅速被丢弃的对象，模拟短暂对象
type temporaryObject struct {
	data []byte
}

// 一组长期存活的对象，模拟老一代对象
var longLivedObjects []*temporaryObject

// 创建并管理一个临时对象
func createObject(objSize int, longLivedRatio float64) {
	// 创建指定大小的临时对象
	obj := &temporaryObject{
		data: make([]byte, objSize),
	}

	// 随机填充一些数据来确保内存被实际使用
	rand.Read(obj.data)

	// 增加分配对象计数
	allocObjects.Inc()

	// 按照指定比例保留一部分对象模拟长期存活对象
	if rand.Float64() < longLivedRatio {
		longLivedObjects = append(longLivedObjects, obj)

		// 防止长期存活对象列表无限增长
		if len(longLivedObjects) > 10000 {
			// 随机移除一些老对象
			cutoff := rand.Intn(len(longLivedObjects) / 2)
			longLivedObjects = longLivedObjects[cutoff:]
		}
	}
}

// 生成固定负载 - 模拟固定流量
func simulateGCPressure(objSize, allocationRate int, longLivedRatio float64) {
	log.Printf("开始模拟 GC 压力: 对象大小=%d 字节, 分配速率=%d 对象/秒, 长期存活比例=%.2f",
		objSize, allocationRate, longLivedRatio)

	ticker := time.NewTicker(time.Second / time.Duration(allocationRate))
	defer ticker.Stop()

	for range ticker.C {
		createObject(objSize, longLivedRatio)
	}
}

// 生成波动负载 - 模拟日常流量波动
func simulateWaveLoad(objSize int, allocationRate int, longLivedRatio float64) {
	log.Println("开始模拟波动负载...")
	cycleTime := 10 * time.Second // 一个波动周期
	ticker := time.NewTicker(time.Second / time.Duration(allocationRate))
	defer ticker.Stop()

	for range ticker.C {
		// 计算当前周期中的位置(0-1)
		cycle := float64(time.Now().UnixNano()/1e6) / float64(cycleTime.Milliseconds())
		position := cycle - float64(int(cycle))

		// 正弦波：调整分配频率
		rate := 0.1 + 0.9*((math.Sin(position*2*math.Pi)+1)/2)
		if rand.Float64() < rate {
			createObject(objSize, longLivedRatio)
		}
	}
}

// 生成尖刺负载 - 模拟突发流量
func simulateSpikeLoad(objSize, allocationRate int, longLivedRatio float64) {
	log.Println("开始模拟尖刺负载...")
	ticker := time.NewTicker(time.Second / time.Duration(allocationRate))
	defer ticker.Stop()

	for range ticker.C {
		// 80%时间保持低负载
		if rand.Float64() < 0.8 {
			createObject(objSize, longLivedRatio)
		} else {
			// 20%时间产生突发流量
			spikeStart := time.Now()
			spikeDuration := time.Duration(1+rand.Intn(3)) * time.Second

			log.Printf("产生负载尖刺! 持续%v", spikeDuration)

			// 快速分配大量对象
			for time.Now().Sub(spikeStart) < spikeDuration {
				createObject(objSize, longLivedRatio)
				time.Sleep(50 * time.Millisecond)
			}

			// 尖刺后短暂休息
			time.Sleep(2 * time.Second)
		}
	}
}

func main() {
	// 命令行参数
	port := flag.Int("port", 8080, "HTTP 服务端口")
	gcPercent := flag.Int("gogc", 100, "GOGC 值")
	objSize := flag.Int("obj-size", 1024, "对象大小 (字节)")
	allocationRate := flag.Int("alloc-rate", 1000, "对象分配速率 (对象/秒)")
	longLivedRatio := flag.Float64("long-lived", 0.05, "长期存活对象比例 (0.0-1.0)")
	loadType := flag.String("load-type", "constant", "负载类型: constant(固定), wave(波动), spike(尖刺)")
	memlimit := flag.Int("memlimit", 0, "内存限制 (MB), 默认不限制")
	ballast := flag.Int("ballast", 0, " ballast 大小 (MB), 默认不限制")
	flag.Parse()

	// 设置 GOGC 值
	if gcPercent != nil {
		// max gcPercent = (maxMem*0.7 - liveheapmem) / liveheapmem * 100
		debug.SetGCPercent(*gcPercent)
	}
	if memlimit != nil && *memlimit > 0 {
		log.Printf("设置内存限制: %d MB", *memlimit)
		debug.SetMemoryLimit(int64(*memlimit * 1024 * 1024))
	}
	if ballast != nil && *ballast > 0 {
		log.Printf("设置 ballast: %d MB", *ballast)
		ballastBucket := make([]byte, *ballast*1024*1024)
		runtime.KeepAlive(ballastBucket)
	}
	log.Printf("GOGC 设置为: %d", *gcPercent)

	// 启动 HTTP 服务
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "GC 压测服务运行中.\n\n")
		fmt.Fprintf(w, "- 访问 /metrics 获取 Prometheus 指标\n")
		fmt.Fprintf(w, "- 访问 /debug/pprof/ 获取性能分析数据\n")
		fmt.Fprintf(w, "- 访问 /debug/pprof/heap 查看内存分配情况\n")
		fmt.Fprintf(w, "- 访问 /debug/pprof/goroutine 查看 goroutine 信息\n")
	})

	// 根据指定负载类型启动对应的模拟函数
	switch *loadType {
	case "constant":
		go simulateGCPressure(*objSize, *allocationRate, *longLivedRatio)
	case "wave":
		go simulateWaveLoad(*objSize, *allocationRate, *longLivedRatio)
	case "spike":
		go simulateSpikeLoad(*objSize, *allocationRate, *longLivedRatio)
	default:
		log.Printf("未知的负载类型: %s, 使用默认的固定负载", *loadType)
		go simulateGCPressure(*objSize, *allocationRate, *longLivedRatio)
	}

	// 输出当前信息
	log.Printf("当前 GOGC 值: %d", *gcPercent)
	log.Printf("负载类型: %s", *loadType)
	log.Printf("HTTP 服务启动在 :%d", *port)
	log.Printf("性能分析服务: http://localhost:%d/debug/pprof/", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
