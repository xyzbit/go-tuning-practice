package main

import (
	"flag"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/xyzbit/go-tuning-practice/gogctuner"
)

// 内存对象结构
type memoryObject struct {
	data      []byte
	createdAt time.Time
}

// 全局变量，保存内存对象
var (
	globalObjects    []*memoryObject
	dataHoldDuration time.Duration
	mu               sync.Mutex
)

func main() {
	// 命令行参数
	memLimitMB := flag.Int("mem-limit", 500, "内存限制(MB)，模拟容器限制")
	enableTuner := flag.Bool("enable-tuner", true, "是否启用GOGCTuner")
	loadPattern := flag.String("load", "wave", "负载模式: constant|wave|spike")
	minObjSizeMB := flag.Int("min-obj", 1, "最小对象大小(MB)")
	maxObjSizeMB := flag.Int("max-obj", 10, "最大对象大小(MB)")
	duration := flag.Int("duration", 60, "测试持续时间(秒)")
	holdTime := flag.Int("hold", 5, "对象保留时间(秒)")
	debugMode := flag.Bool("debug", true, "是否启用调试模式")
	flag.Parse()

	// 设置内存对象保留时间
	dataHoldDuration = time.Duration(*holdTime) * time.Second

	// 设置内存限制环境变量（GOGCTuner会读取）
	memLimitBytes := int64(*memLimitMB) * 1024 * 1024
	os.Setenv("MEMORY_LIMIT_BYTES", strconv.FormatInt(memLimitBytes, 10))

	// 输出配置信息
	log.Printf("测试配置: 内存限制=%dMB, 启用调优=%v, 负载模式=%s, 对象大小=%d-%dMB, 持续=%d秒",
		*memLimitMB, *enableTuner, *loadPattern, *minObjSizeMB, *maxObjSizeMB, *duration)

	// 初始化调优器
	if *enableTuner {
		log.Println("启动GOGCTuner...")
		tunerConfig := gogctuner.Config{
			MemoryHardLimit:   memLimitBytes,
			SafetyFactor:      0.7,
			MinGOGC:           25,
			MaxGOGC:           500,
			AllowPeakOverride: true,
			PeakThreshold:     1.5,
			DebugMode:         *debugMode,
		}

		tuner, err := gogctuner.NewTuner(tunerConfig)
		if err != nil {
			log.Fatalf("GOGCTuner初始化失败: %v", err)
		}
		tuner.Start()
		defer tuner.Stop()

		// 启动指标报告协程
		go reportMetrics(tuner)
	} else {
		log.Println("使用默认GOGC=100")
	}

	// 启动清理协程
	go cleanupOldObjects()

	// 根据负载模式生成内存压力
	switch *loadPattern {
	case "constant":
		// 恒定负载
		generateConstantLoad(*minObjSizeMB, *maxObjSizeMB, *duration)
	case "wave":
		// 波动负载
		generateWaveLoad(*minObjSizeMB, *maxObjSizeMB, *duration)
	case "spike":
		// 尖刺负载
		generateSpikeLoad(*minObjSizeMB, *maxObjSizeMB, *duration)
	default:
		log.Fatalf("未知负载模式: %s", *loadPattern)
	}

	log.Println("测试完成")
}

// 生成恒定负载
func generateConstantLoad(minSizeMB, maxSizeMB, durationSec int) {
	log.Println("开始生成恒定负载...")
	endTime := time.Now().Add(time.Duration(durationSec) * time.Second)

	for time.Now().Before(endTime) {
		allocateRandomObject(minSizeMB, maxSizeMB)
		time.Sleep(200 * time.Millisecond) // 稳定分配速率
	}
}

// 生成波动负载 - 模拟日常流量波动
func generateWaveLoad(minSizeMB, maxSizeMB, durationSec int) {
	log.Println("开始生成波动负载...")
	endTime := time.Now().Add(time.Duration(durationSec) * time.Second)
	cycleTime := 10 * time.Second // 一个波动周期

	for time.Now().Before(endTime) {
		// 计算当前周期中的位置(0-1)
		cycle := float64(time.Now().UnixNano()/1e6) / float64(cycleTime.Milliseconds())
		position := cycle - float64(int(cycle))

		// 正弦波：调整分配频率
		rate := 0.1 + 0.9*((math.Sin(position*2*math.Pi)+1)/2)
		sleepTime := time.Duration((1-rate)*1000) * time.Millisecond

		// 按波形分配对象
		allocateRandomObject(minSizeMB, maxSizeMB)
		time.Sleep(sleepTime)
	}
}

// 生成尖刺负载 - 模拟突发流量
func generateSpikeLoad(minSizeMB, maxSizeMB, durationSec int) {
	log.Println("开始生成尖刺负载...")
	endTime := time.Now().Add(time.Duration(durationSec) * time.Second)

	for time.Now().Before(endTime) {
		// 80%时间保持低负载
		if rand.Float64() < 0.8 {
			allocateRandomObject(minSizeMB, minSizeMB+1)
			time.Sleep(500 * time.Millisecond)
		} else {
			// 20%时间产生突发流量
			spikeStart := time.Now()
			spikeDuration := time.Duration(1+rand.Intn(3)) * time.Second

			log.Printf("产生负载尖刺! 持续%v", spikeDuration)

			// 快速分配大量对象
			for time.Now().Sub(spikeStart) < spikeDuration {
				allocateRandomObject(maxSizeMB-2, maxSizeMB)
				time.Sleep(50 * time.Millisecond)
			}

			// 尖刺后短暂休息
			time.Sleep(2 * time.Second)
		}
	}
}

// 分配随机大小的对象
func allocateRandomObject(minSizeMB, maxSizeMB int) {
	sizeMB := minSizeMB
	if maxSizeMB > minSizeMB {
		sizeMB = minSizeMB + rand.Intn(maxSizeMB-minSizeMB)
	}

	sizeBytes := sizeMB * 1024 * 1024

	// 创建并填充随机数据
	data := make([]byte, sizeBytes)
	rand.Read(data) // 填充随机值

	object := &memoryObject{
		data:      data,
		createdAt: time.Now(),
	}

	// 将对象添加到全局列表
	mu.Lock()
	globalObjects = append(globalObjects, object)
	mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	log.Printf("分配 %dMB 对象, 当前堆内存: %dMB, 对象数: %d",
		sizeMB, memStats.HeapAlloc>>20, len(globalObjects))
}

// 清理过期对象
func cleanupOldObjects() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("开始运行清理过期对象协程...")

	for range ticker.C {
		mu.Lock()
		now := time.Now()
		newList := make([]*memoryObject, 0, len(globalObjects))

		removed := 0
		for _, obj := range globalObjects {
			if now.Sub(obj.createdAt) < dataHoldDuration {
				newList = append(newList, obj)
			} else {
				removed++
				// 显式置空帮助GC
				obj.data = nil
			}
		}

		if removed > 0 {
			log.Printf("清理 %d 个过期对象", removed)
		}

		globalObjects = newList
		mu.Unlock()
	}
}

// 周期性报告内存和GC指标
func reportMetrics(tuner *gogctuner.Tuner) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	log.Println("开始运行指标报告协程...")

	var lastGC uint32 = 0
	var lastPauseNs uint64 = 0

	for range ticker.C {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// 计算GC耗时
		var gcCPUTime float64 = 0
		if memStats.NumGC > lastGC {
			gcPauseTotal := uint64(0)
			// 计算新的GC暂停总时间
			for i := lastGC + 1; i <= memStats.NumGC && i <= lastGC+255; i++ {
				idx := i % 256
				gcPauseTotal += memStats.PauseNs[idx]
			}

			// 计算增量GC暂停时间
			if gcPauseTotal > lastPauseNs {
				gcPauseDelta := gcPauseTotal - lastPauseNs
				gcCPUTime = float64(gcPauseDelta) / float64(1000000) // 转换为毫秒
			}

			lastGC = memStats.NumGC
			lastPauseNs = gcPauseTotal
		}

		metrics := tuner.GetMetrics()
		log.Printf("指标报告 - GOGC: %d, 堆内存: %dMB, 对象数: %d, GC次数: %d, 内存使用率: %.2f%%, GC耗时: %.2fms",
			metrics["current_gogc"], memStats.HeapAlloc>>20, memStats.HeapObjects,
			memStats.NumGC, metrics["memory_usage_ratio"].(float64)*100, gcCPUTime)
	}
}
