package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof" // 导入 pprof，它会自动注册 HTTP 处理程序
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/process"
)

// 定义 prometheus 指标
var allocObjects = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "alloc_objects_total",
		Help: "分配对象总数",
	},
)

var (
	processCPUPercent = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "process_cpu_usage_percent",
			Help: "CPU percentage used by the Go process",
		},
	)

	processMemoryBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "process_memory_bytes",
			Help: "Memory used by the Go process in bytes",
		},
	)
)

// 添加请求延迟相关的指标定义
var (
	requestDuration = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "http_request_duration_seconds_summary",
			Help:       "HTTP请求延迟(秒)",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"handler", "status"},
	)
)

// 创建一个包装中间件来记录请求延迟
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 调用包装的处理函数
		recorder := &statusRecorder{
			ResponseWriter: w,
			Status:         200,
		}
		next.ServeHTTP(recorder, r)

		// 记录请求延迟
		duration := time.Since(start).Seconds()
		requestDuration.WithLabelValues(
			r.URL.Path,
			fmt.Sprintf("%d", recorder.Status),
		).Observe(duration)
	})
}

// 用于捕获状态码的响应写入器
type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func collectProcessMetrics() {
	pid := os.Getpid()
	log.Printf("当前进程ID: %d", pid)
	proc, _ := process.NewProcess(int32(pid))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if cpuPercent, err := proc.CPUPercent(); err == nil {
			processCPUPercent.Set(cpuPercent)
		}

		if memInfo, err := proc.MemoryInfo(); err == nil {
			processMemoryBytes.Set(float64(memInfo.RSS))
		}
	}
}

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

func main() {
	// 命令行参数
	port := flag.Int("port", 8080, "HTTP 服务端口")
	gcPercent := flag.Int("gogc", 100, "GOGC 值")
	objSize := flag.Int("obj-size", 1024, "对象大小 (字节)")
	longLivedRatio := flag.Float64("long-lived", 0.05, "长期存活对象比例 (0.0-1.0)")
	memlimit := flag.Int("memlimit", 0, "内存限制 (MB), 默认不限制")
	ballast := flag.Int("ballast", 0, " ballast 大小 (MB), 默认不限制")
	flag.Parse()

	// 在main函数中启动
	go collectProcessMetrics()

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
	http.Handle("/", metricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, "GC 压测服务运行中.\n\n")
		fmt.Fprintf(w, "- 访问 /metrics 获取 Prometheus 指标\n")
		fmt.Fprintf(w, "- 访问 /debug/pprof/ 获取性能分析数据\n")
		fmt.Fprintf(w, "- 访问 /debug/pprof/heap 查看内存分配情况\n")
		fmt.Fprintf(w, "- 访问 /debug/pprof/goroutine 查看 goroutine 信息\n")

		time.Sleep(10 * time.Millisecond)

		createObject(*objSize, *longLivedRatio)
	})))

	// 根据指定负载类型启动对应的模拟函数
	// switch *loadType {
	// case "constant":
	// 	go simulateGCPressure(*objSize, *allocationRate, *longLivedRatio)
	// case "wave":
	// 	go simulateWaveLoad(*objSize, *allocationRate, *longLivedRatio)
	// case "spike":
	// 	go simulateSpikeLoad(*objSize, *allocationRate, *longLivedRatio)
	// default:
	// 	log.Printf("未知的负载类型: %s, 使用默认的固定负载", *loadType)
	// 	go simulateGCPressure(*objSize, *allocationRate, *longLivedRatio)
	// }

	// 输出当前信息
	log.Printf("当前 GOGC 值: %d", *gcPercent)
	log.Printf("HTTP 服务启动在 :%d", *port)
	log.Printf("性能分析服务: http://localhost:%d/debug/pprof/", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
