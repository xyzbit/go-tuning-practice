package gogctuner

import (
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
)

const (
	// 默认安全系数
	defaultSafetyFactor = 0.7
	// 强制GC时间间隔（保持与Go运行时一致）
	forcedGCInterval = 2 * time.Minute
)

// Config 调优器配置
type Config struct {
	// 内存硬限制(字节)，0表示自动读取cgroup限制
	MemoryHardLimit int64
	// 安全系数(0-1)，控制最大允许内存使用比例
	SafetyFactor float64
	// 最小GOGC值限制
	MinGOGC int
	// 最大GOGC值限制
	MaxGOGC int
	// 是否允许临时突破限制
	AllowPeakOverride bool
	// 突破阈值倍数
	PeakThreshold float64
	// 调试模式
	DebugMode bool
}

// Tuner GC调优器
type Tuner struct {
	config       Config
	mu           sync.Mutex
	currentGOGC  int
	lastGCTime   time.Time
	memoryLimit  int64
	enabled      bool
	forceGCTimer *time.Timer
}

// NewTuner 创建新的调优器
func NewTuner(config Config) (*Tuner, error) {
	// 应用默认值
	if config.SafetyFactor <= 0 || config.SafetyFactor > 1 {
		config.SafetyFactor = defaultSafetyFactor
	}

	if config.MinGOGC <= 0 {
		config.MinGOGC = 25 // 最小值避免GC太频繁
	}

	if config.MaxGOGC <= 0 {
		config.MaxGOGC = 500 // 最大值避免内存占用过高
	}

	if !config.AllowPeakOverride {
		config.PeakThreshold = 1.0
	} else if config.PeakThreshold < 1.0 {
		config.PeakThreshold = 1.5
	}

	memLimit := config.MemoryHardLimit
	if memLimit == 0 {
		// 真实场景中应该读取cgroup内存限制
		// 这里使用环境变量模拟容器内存限制
		if envLimit := os.Getenv("MEMORY_LIMIT_BYTES"); envLimit != "" {
			if parsed, err := strconv.ParseInt(envLimit, 10, 64); err == nil {
				memLimit = parsed
			}
		}

		// 如果没有设置环境变量，使用系统内存的一部分作为模拟值
		if memLimit == 0 {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			memLimit = int64(float64(memStats.Sys) * 0.8) // 使用当前申请内存的80%作为限制
		}
	}

	// 读取当前GOGC值
	currentGOGC := 100 // 默认GOGC
	if envGOGC := os.Getenv("GOGC"); envGOGC != "" {
		if parsed, err := strconv.Atoi(envGOGC); err == nil {
			currentGOGC = parsed
		}
	}

	tuner := &Tuner{
		config:      config,
		currentGOGC: currentGOGC,
		lastGCTime:  time.Now(),
		memoryLimit: memLimit,
		enabled:     true,
	}

	if config.DebugMode {
		log.Printf("GOGCTuner初始化: 内存限制=%d字节, 安全系数=%.2f, 当前GOGC=%d",
			memLimit, config.SafetyFactor, currentGOGC)
	}

	return tuner, nil
}

// Start 启动调优循环
func (t *Tuner) Start() {
	if !t.enabled {
		t.enabled = true
	}

	// 设置强制GC定时器
	t.forceGCTimer = time.AfterFunc(forcedGCInterval, func() {
		t.adjustGOGC()
		runtime.GC()
		t.forceGCTimer.Reset(forcedGCInterval)
	})

	// 设置finalizer观察GC事件
	runtime.SetFinalizer(new(byte), func(_ *byte) {
		t.adjustGOGC()
		// 重新设置finalizer保持监听
		runtime.SetFinalizer(new(byte), func(_ *byte) {
			t.adjustGOGC()
		})
	})

	// 立即进行首次调整，但在goroutine中异步执行避免死锁
	t.adjustGOGC()
}

// Stop 停止调优
func (t *Tuner) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.forceGCTimer != nil {
		t.forceGCTimer.Stop()
	}
	t.enabled = false

	// 恢复默认GOGC
	debug.SetGCPercent(100)
	if t.config.DebugMode {
		log.Println("GOGCTuner已停止，GOGC恢复为默认值100")
	}
}

// GetCurrentGOGC 获取当前GOGC值
func (t *Tuner) GetCurrentGOGC() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentGOGC
}

// adjustGOGC 核心算法：根据当前内存占用调整GOGC
func (t *Tuner) adjustGOGC() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled {
		return
	}

	// 记录GC间隔
	now := time.Now()
	gcInterval := now.Sub(t.lastGCTime)
	t.lastGCTime = now

	// 读取当前内存状态
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 核心指标：存活对象占用内存
	liveBytes := memStats.HeapAlloc

	// 计算可用内存上限
	safetyLimit := float64(t.memoryLimit) * t.config.SafetyFactor
	peakLimit := float64(t.memoryLimit) * t.config.SafetyFactor * t.config.PeakThreshold

	// 计算新的GOGC值
	// GOGC = (可用内存 / 存活对象大小 - 1) * 100
	var newGOGC int
	if liveBytes > 0 {
		if float64(liveBytes) > safetyLimit {
			// 存活对象已超过安全限制，降低GOGC以更频繁GC
			newGOGC = t.config.MinGOGC
		} else if t.config.AllowPeakOverride && float64(liveBytes) < safetyLimit*0.5 {
			// 存活对象远低于安全限制，可以适当提高GOGC
			maxAvailableBytes := peakLimit
			newGOGC = int((maxAvailableBytes/float64(liveBytes) - 1) * 100)
		} else {
			// 正常计算
			maxAvailableBytes := safetyLimit
			newGOGC = int((maxAvailableBytes/float64(liveBytes) - 1) * 100)
		}

		// 应用上下限
		if newGOGC < t.config.MinGOGC {
			newGOGC = t.config.MinGOGC
		} else if newGOGC > t.config.MaxGOGC {
			newGOGC = t.config.MaxGOGC
		}
	} else {
		// 异常情况，使用默认值
		newGOGC = 100
	}

	// 仅当变化超过10%时才更新
	if newGOGC != t.currentGOGC && (float64(newGOGC-t.currentGOGC)/float64(t.currentGOGC) > 0.1 ||
		float64(t.currentGOGC-newGOGC)/float64(t.currentGOGC) > 0.1) {
		t.currentGOGC = newGOGC
		debug.SetGCPercent(newGOGC)

		if t.config.DebugMode {
			memUsageRatio := float64(liveBytes) / float64(t.memoryLimit)
			log.Printf("GOGCTuner: 调整GOGC=%d, 存活对象=%dMB, 内存限制=%dMB, 占比=%.2f%%, GC间隔=%v",
				newGOGC, liveBytes>>20, t.memoryLimit>>20, memUsageRatio*100, gcInterval)
		}
	}
}

// GetMetrics 获取调优器指标（用于监控）
func (t *Tuner) GetMetrics() map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"current_gogc":       t.currentGOGC,
		"memory_limit_bytes": t.memoryLimit,
		"heap_alloc_bytes":   memStats.HeapAlloc,
		"heap_objects":       memStats.HeapObjects,
		"gc_cycles":          memStats.NumGC,
		"memory_usage_ratio": float64(memStats.HeapAlloc) / float64(t.memoryLimit),
		"safety_factor":      t.config.SafetyFactor,
		"tuner_enabled":      t.enabled,
	}
}
