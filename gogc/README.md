# GOGC 测试工具

这个工具用于模拟 GC 不友好的代码并进行性能测试，同时将 GC 相关指标导出到 Prometheus 进行监控。

## 功能特点

- 模拟产生大量短生命周期对象，触发频繁 GC
- 可控制对象大小、分配速率和长期存活对象比例
- 支持通过命令行参数调整 GOGC 值
- 提供多种负载模式：固定负载、波动负载和尖刺负载
- 使用标准 Go 性能分析工具 pprof 导出指标
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

## Web 接口

运行服务后可以访问以下接口：

- `/` - 服务首页，提供链接导航
- `/metrics` - Prometheus 指标采集接口
- `/debug/pprof/` - Go pprof 性能分析接口
- `/debug/pprof/heap` - 内存分配情况分析
- `/debug/pprof/goroutine` - goroutine 分析
- `/debug/pprof/allocs` - 内存分配分析
- `/debug/pprof/mutex` - 锁竞争分析

## 常用 Prometheus 查询语句

### GC 基本指标

```
# GC 执行次数 (每秒)
rate(go_gc_cycles_total[1m])

# GC 暂停时间 (毫秒)
rate(go_gc_pause_total_ns[1m]) / 1000000

# 堆内存分配情况 (MB)
go_memstats_heap_alloc_bytes / 1024 / 1024

# 堆对象数量
go_memstats_heap_objects

# 分配对象速率 (只支持自定义指标)
rate(alloc_objects_total[1m])
```

### GC 效率分析

```
# GC CPU 使用率估算
rate(go_gc_pause_total_ns[1m]) / 1000000000

# 每次 GC 回收的内存量估算 (MB)
(rate(go_memstats_heap_alloc_bytes[1m]) / rate(go_gc_cycles_total[1m])) / 1024 / 1024

# GC 触发 (次/秒)
go_gc_duration_seconds_count
```

### 内存使用趋势

```
# 堆内存使用率
go_memstats_heap_inuse_bytes / go_memstats_heap_sys_bytes

# 下一次 GC 触发点 (MB)
go_memstats_next_gc_bytes / 1024 / 1024

# 内存分配速率 (MB/s)
rate(go_memstats_alloc_bytes_total[1m]) / 1024 / 1024
```

## 使用 pprof 进行分析

除了在 web 界面查看指标外，还可以使用命令行工具进行分析：

```bash
# 获取 30 秒的 CPU 性能数据
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# 查看内存分配情况
go tool pprof http://localhost:8080/debug/pprof/heap

# 查看 goroutine 阻塞情况
go tool pprof http://localhost:8080/debug/pprof/block
```

## 测试路径
在一下三种情况下分别测试：
- **固定负载** - 最基本的压测模式，适合测试基准性能和调优参数
- **波动负载** - 更接近真实世界的应用场景，适合测试 GC 对动态变化流量的适应性
- **尖刺负载** - 测试系统在突发流量下的 GC 行为，适合评估系统在极端条件下的稳定性

统一的测试用例：
1. ./gogc_test -obj-size=4096 优先使用默认参数
2. ./gogc_test -obj-size=4096 -gogc=200 增加gog查看变化
3. ./gogc_test -obj-size=4096 -gogc=200 -ballast=100 增加ballast=100MB查看变化
4. ./gogc_test -obj-size=4096 -gogc=200 -ballast=100 -memlimit=250 增加memlimit=250MB查看变化