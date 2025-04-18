# GOGC 测试工具

这个工具用于模拟 GC 不友好的代码并进行性能测试，同时将 GC 相关指标导出到 Prometheus 进行监控。

## 功能特点

- 模拟产生大量短生命周期对象，触发频繁 GC
- 可控制对象大小、分配速率和长期存活对象比例
- 支持通过命令行参数调整 GOGC 值
- 提供多种负载模式：固定负载、波动负载和尖刺负载
- 导出关键 GC 指标到 Prometheus
- 包含开箱即用的 Prometheus 和 Grafana 部署配置
- 提供常用的 GC 状态查询语句

## 快速开始

### 编译运行

```bash
# 编译
go build -o gogc_test

# 运行 (使用默认参数)
./gogc_test

# 使用自定义参数运行
./gogc_test -gogc=200 -obj-size=2048 -alloc-rate=2000 -long-lived=0.1 -port=8080

# 使用波动负载模式运行
./gogc_test -load-type=wave -obj-size=2048 -long-lived=0.1
```

### 命令行参数

- `-gogc` - 设置 GOGC 百分比值 (默认 100)
- `-obj-size` - 每个对象的大小 (字节, 默认 1024)
- `-alloc-rate` - 对象分配速率 (对象/秒, 默认 1000)
- `-long-lived` - 长期存活对象比例 (0.0-1.0, 默认 0.05)
- `-port` - HTTP 服务端口 (默认 8080)
- `-load-type` - 负载类型 (默认 constant)
  - `constant`: 固定负载 - 按固定速率分配对象
  - `wave`: 波动负载 - 模拟日常波动流量，以正弦波形式变化
  - `spike`: 尖刺负载 - 模拟突发流量，大部分时间保持低负载，偶尔产生尖刺

## 启动 Prometheus 和 Grafana

该项目包含一个 docker-compose 配置，用于启动 Prometheus 和 Grafana：

```bash
# 启动 Prometheus 和 Grafana
docker-compose up -d

# 查看容器状态
docker-compose ps

# 停止服务
docker-compose down
```

- Prometheus 界面访问: http://localhost:9090
- Grafana 界面访问: http://localhost:3000 (用户名/密码: admin/admin)

## 常用 Prometheus 查询语句

### GC 基本指标

```
# GC 执行次数 (每秒)
rate(gc_count_total[1m])

# GC 暂停时间 (毫秒)
gc_pause_ns / 1000000

# 堆内存分配情况 (MB)
go_memory_stats{type="heap_alloc"} / 1024 / 1024

# 堆对象数量
go_memory_stats{type="heap_objects"}

# 分配对象速率 (每秒)
rate(alloc_objects_total[1m])
```

### GC 效率分析

```
# GC CPU 使用率估算
rate(gc_pause_ns[1m]) / 1000000000

# 每次 GC 回收的内存量估算 (MB)
rate(go_memory_stats{type="heap_alloc"}[1m]) / rate(gc_count_total[1m]) / 1024 / 1024

# GC 触发频率 (秒/次)
1 / rate(gc_count_total[1m])

# 对象分配速率与 GC 运行次数比值
rate(alloc_objects_total[1m]) / rate(gc_count_total[1m])
```

### 内存使用趋势

```
# 堆内存使用率
go_memory_stats{type="heap_inuse"} / go_memory_stats{type="heap_sys"}

# 下一次 GC 触发点 (MB)
go_memory_stats{type="next_gc"} / 1024 / 1024

# 内存分配速率 (MB/s)
rate(go_memory_stats{type="alloc"}[1m]) / 1024 / 1024
```

## 负载模式比较

- **固定负载** - 最基本的压测模式，适合测试基准性能和调优参数
- **波动负载** - 更接近真实世界的应用场景，适合测试 GC 对动态变化流量的适应性
- **尖刺负载** - 测试系统在突发流量下的 GC 行为，适合评估系统在极端条件下的稳定性

## 调优建议

1. **增加 GOGC 值**：默认值为 100，表示当堆大小增加 100% 时触发 GC。增加此值可减少 GC 频率，但会增加内存使用。

2. **优化对象分配**：减少短生命周期对象的创建，特别是在热路径上。

3. **对象池化**：对于频繁创建和销毁的对象，考虑使用 sync.Pool。

4. **控制长生命周期对象**：减少长生命周期对象的创建，它们会增加 GC 扫描时间。

5. **内存预分配**：使用合适容量的切片和映射，避免频繁扩容。 