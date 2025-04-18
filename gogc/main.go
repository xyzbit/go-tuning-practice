package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// 定义 prometheus 指标
	gcCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gc_count_total",
			Help: "GC 执行次数",
		},
	)
	gcPauseNs = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gc_pause_ns",
			Help: "最近一次 GC 暂停时间 (纳秒)",
		},
	)
	memStats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "go_memory_stats",
			Help: "Go 内存统计信息",
		},
		[]string{"type"},
	)
	allocObjects = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "alloc_objects_total",
			Help: "分配对象总数",
		},
	)
)

func init() {
	// 注册指标到 prometheus
	prometheus.MustRegister(gcCount)
	prometheus.MustRegister(gcPauseNs)
	prometheus.MustRegister(memStats)
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
func simulateWaveLoad(objSize int, longLivedRatio float64) {
	log.Println("开始模拟波动负载...")
	cycleTime := 10 * time.Second // 一个波动周期
	ticker := time.NewTicker(200 * time.Millisecond)
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
func simulateSpikeLoad(objSize int, longLivedRatio float64) {
	log.Println("开始模拟尖刺负载...")
	ticker := time.NewTicker(500 * time.Millisecond)
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

func collectMetrics() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastGC uint32
	var m runtime.MemStats

	for range ticker.C {
		runtime.ReadMemStats(&m)

		// 记录 GC 相关指标
		gcCount.Add(float64(m.NumGC - lastGC))
		lastGC = m.NumGC

		if m.NumGC > 0 {
			gcPauseNs.Set(float64(m.PauseNs[(m.NumGC-1)%256]))
		}

		// 记录内存使用情况
		memStats.WithLabelValues("alloc").Set(float64(m.Alloc))
		memStats.WithLabelValues("sys").Set(float64(m.Sys))
		memStats.WithLabelValues("heap_alloc").Set(float64(m.HeapAlloc))
		memStats.WithLabelValues("heap_sys").Set(float64(m.HeapSys))
		memStats.WithLabelValues("heap_idle").Set(float64(m.HeapIdle))
		memStats.WithLabelValues("heap_inuse").Set(float64(m.HeapInuse))
		memStats.WithLabelValues("heap_released").Set(float64(m.HeapReleased))
		memStats.WithLabelValues("heap_objects").Set(float64(m.HeapObjects))
		memStats.WithLabelValues("next_gc").Set(float64(m.NextGC))
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
	flag.Parse()

	// 设置 GOGC 值
	debug.SetGCPercent(*gcPercent)
	log.Printf("GOGC 设置为: %d", *gcPercent)

	// 启动指标收集
	go collectMetrics()

	// 启动 HTTP 服务
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "GC 压测服务运行中. 访问 /metrics 获取 Prometheus 指标。")
	})

	// 根据指定负载类型启动对应的模拟函数
	switch *loadType {
	case "constant":
		go simulateGCPressure(*objSize, *allocationRate, *longLivedRatio)
	case "wave":
		go simulateWaveLoad(*objSize, *longLivedRatio)
	case "spike":
		go simulateSpikeLoad(*objSize, *longLivedRatio)
	default:
		log.Printf("未知的负载类型: %s, 使用默认的固定负载", *loadType)
		go simulateGCPressure(*objSize, *allocationRate, *longLivedRatio)
	}

	// 输出当前信息
	log.Printf("当前 GOGC 值: %d", *gcPercent)
	log.Printf("负载类型: %s", *loadType)
	log.Printf("HTTP 服务启动在 :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
