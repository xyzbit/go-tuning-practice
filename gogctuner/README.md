# GOGCTuner - Go GC 自动调优库

GOGCTuner 是一个基于 [Uber GC 调优策略](https://eng.uber.com/how-we-saved-70k-cores-across-30-mission-critical-services/)的Go语言GC自动调优库，用于动态优化GC行为，降低CPU使用率，同时避免OOM风险。

## 核心功能

- **动态GOGC调整**：根据实际内存使用情况自动调整GOGC值
- **容器感知**：自动读取并遵守容器内存限制
- **OOM保护**：通过安全系数机制避免内存溢出风险
- **低开销监控**：使用Go Finalizer机制实现轻量级GC事件监控
- **高峰流量适配**：支持临时突破内存限制以应对流量高峰

## 快速开始

1. 创建并启动调优器
```go
// 创建配置
config := gogctuner.Config{
    SafetyFactor:      0.7,  // 使用内存限制的70%
    AllowPeakOverride: true, // 允许高峰期突破限制
    DebugMode:         true, // 开启调试日志
}

// 初始化调优器
tuner, err := gogctuner.NewTuner(config)
if err != nil {
    log.Fatalf("初始化GOGCTuner失败: %v", err)
}

// 启动调优
tuner.Start()
defer tuner.Stop() // 程序结束时停止调优
```

## 配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| MemoryHardLimit | int64 | 0 | 内存硬限制(字节)，0表示自动读取cgroup限制 |
| SafetyFactor | float64 | 0.7 | 安全系数(0-1)，控制最大允许内存使用比例 |
| MinGOGC | int | 25 | 最小GOGC值限制 |
| MaxGOGC | int | 500 | 最大GOGC值限制 |
| AllowPeakOverride | bool | false | 是否允许临时突破限制 |
| PeakThreshold | float64 | 1.5 | 突破阈值倍数 |
| DebugMode | bool | false | 调试模式，输出详细日志 |

## 使用场景

- 微服务容器化部署
- 计算密集型服务，减少GC对CPU的占用
- 内存使用率较高的服务，避免OOM风险
- 流量波动较大的服务，动态适应负载变化

## 实验示例

项目包含一个内存压力测试程序，用于展示GOGCTuner在不同负载模式下的效果：

```bash
# 切换到示例目录
cd example

# 运行波动负载测试（默认）
go run memory_stress.go

# 运行突发负载测试
go run memory_stress.go -load spike -duration 120

# 对比未启用调优的效果
go run memory_stress.go -enable-tuner=false
```

## 监控指标

调优器提供了丰富的监控指标，可通过`GetMetrics()`方法获取：

```go
metrics := tuner.GetMetrics()
log.Printf("当前GOGC: %d, 内存使用率: %.2f%%", 
    metrics["current_gogc"], metrics["memory_usage_ratio"].(float64)*100)
```

## 注意事项

1. **安全系数选择**：默认0.7适用于大多数场景，但需根据服务特性调整
2. **容器环境**：确保服务能正确读取cgroup内存限制
3. **监控接入**：建议将指标接入Prometheus等监控系统，实时观察调优效果

## 延伸阅读

- [Uber工程博客：如何在30个关键服务中节省70k核心](https://eng.uber.com/how-we-saved-70k-cores-across-30-mission-critical-services/)
- [Go GC指南](https://tip.golang.org/doc/gc-guide)
- [优化策略文档](优化策略.md) 