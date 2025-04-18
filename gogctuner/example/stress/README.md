# GOGCTuner 实验案例

本目录包含一个内存压力测试程序，用于展示 GOGCTuner 在不同内存负载模式下的动态调优效果。

## 实验目标

1. 验证 GOGCTuner 能否根据内存使用情况动态调整 GOGC 值
2. 对比启用/禁用调优器时的 GC 行为差异
3. 测试不同负载模式下 GOGCTuner 的应对策略
4. 模拟容器环境中的内存限制情况

## 实验设计

测试程序通过不同的负载模式模拟真实服务场景：

- **恒定负载 (constant)**: 以固定速率分配内存对象，模拟稳定服务
- **波动负载 (wave)**: 使用正弦波模式变化分配频率，模拟日常波动
- **尖刺负载 (spike)**: 间歇性产生高强度内存分配，模拟突发流量

## 运行方法

```bash
# 基本用法（波动负载模式，启用调优器）
go run memory_stress.go

# 调整内存限制（模拟不同容器规格）
go run memory_stress.go -mem-limit 200

# 选择负载模式
go run memory_stress.go -load constant  # 恒定负载
go run memory_stress.go -load wave      # 波动负载（默认）
go run memory_stress.go -load spike     # 尖刺负载

# 调整对象大小和保留时间
go run memory_stress.go -min-obj 5 -max-obj 20 -hold 10

# 关闭调优器以对比效果
go run memory_stress.go -enable-tuner=false

# 长时间测试
go run memory_stress.go -duration 300 -load spike
```

## 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| -mem-limit | 内存限制(MB) | 500 |
| -enable-tuner | 是否启用GOGCTuner | true |
| -load | 负载模式: constant/wave/spike | wave |
| -min-obj | 最小对象大小(MB) | 1 |
| -max-obj | 最大对象大小(MB) | 10 |
| -duration | 测试持续时间(秒) | 60 |
| -hold | 对象保留时间(秒) | 5 |
| -debug | 是否启用调试日志 | true |

## 实验观察点

运行测试时，建议关注以下关键指标：

1. **GOGC变化**：观察GOGC值如何根据内存变化动态调整
2. **GC频率**：比较启用和禁用调优器时GC发生的频率
3. **内存使用率**：观察内存占用相对于限制的比例
4. **尖峰处理**：在spike模式下，观察系统如何应对突发内存分配

## 结果分析

### 预期结果

1. 启用调优器后，GOGC值应随内存使用增长而降低，随内存使用减少而增加
2. 波动负载下，GOGC应呈现相反的波动趋势
3. 尖刺负载下，GOGC应在尖刺出现时快速降低，之后逐渐恢复
4. 内存使用率应始终保持在安全范围内（不超过memLimit * safetyFactor）

### 性能提升

在实际服务中使用GOGCTuner，预期可获得以下收益：

- CPU使用率降低15-30%（主要减少GC相关开销）
- 内存使用更加高效（根据实际需求动态分配）
- 服务在负载波动下更加稳定（避免OOM风险）

## 进阶实验

1. **长时间运行测试**：使用更长的-duration参数（如3600秒），观察长期稳定性
2. **调整安全系数**：修改tunerConfig中的SafetyFactor，观察对GC行为的影响
3. **关闭高峰突破**：设置AllowPeakOverride=false，对比处理突发流量的差异 