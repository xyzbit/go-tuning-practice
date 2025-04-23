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
  - 配置 Data Source 为 Prometheus，url 填写为 `http://go-tuning-prometheus:9090` 或 `http://宿主机IP:9090`
  - import grafana dashboard 文件 `grafana.json`

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

# 监控 Go 应用指标配置方案

要监控您需要的指标（CPU、内存消耗、GC的CPU占比以及请求延迟），我们需要完善代码并配置正确的 Prometheus 查询。

## 代码完善部分

需要在 main.go 中添加请求延迟监控的代码：

```go
// 在 import 部分添加
import (
    "time"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// 添加请求延迟相关的指标定义
var (
    requestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP请求延迟(秒)",
            Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
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
```

然后在 HTTP 服务配置部分修改代码：

```go
// 将 HTTP 服务配置部分修改为
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
})))

// 对 pprof 处理函数也应用中间件
http.Handle("/debug/pprof/", metricsMiddleware(http.DefaultServeMux))
```

## Prometheus 查询语句

### 1. CPU 消耗监控

```promql
# 当前go进程CPU使用量（百分比）
process_cpu_usage_percent
```

### 2. 内存消耗监控

```promql
# 当前堆内存使用量(bytes)
go_memstats_alloc_bytes{job="gogc_test"}

# 堆内存分配速率(bytes/s)
rate(go_memstats_alloc_bytes[1m])

# 进程内存使用量(bytes)
process_memory_bytes
```

### 3. GC情况

```promql
# GC频率
rate(go_gc_duration_seconds_count{job="gogc_test"}[1m])

# GC 
```

### 4. 请求延迟监控

```promql
# 平均请求延迟(秒)
sum(rate(http_request_duration_seconds_sum{job="gogc_test"}[5m])) by (handler) / 
sum(rate(http_request_duration_seconds_count{job="gogc_test"}[5m])) by (handler)

# 请求延迟P95(秒)
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{job="gogc_test"}[5m])) by (handler, le))

# 请求延迟P99(秒)
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="gogc_test"}[5m])) by (handler, le))
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
```
   ./loadtest -host=localhost -port=8080 -rps=100 -load-type=constant -duration=60
```
- **波动负载** - 更接近真实世界的应用场景，适合测试 GC 对动态变化流量的适应性
```
   ./loadtest -host=localhost -port=8080 -rps=100 -load-type=wave -duration=60
```
- **尖刺负载** - 测试系统在突发流量下的 GC 行为，适合评估系统在极端条件下的稳定性
```
   ./loadtest -host=localhost -port=8080 -rps=100 -load-type=spike -duration=60
```

统一的测试用例：
1. ./gogc_test -obj-size=4096 优先使用默认参数
2. ./gogc_test -obj-size=4096 -gogc=200 增加gog查看变化
3. ./gogc_test -obj-size=4096 -gogc=70 减少gogc查看变化
4. ./gogc_test -obj-size=4096 -gogc=200 -ballast=100 增加ballast=100MB查看变化
5. ./gogc_test -obj-size=4096 -gogc=200 -ballast=100 -memlimit=250 增加memlimit=250MB查看变化

// TODO 热修改 GOGC 和 其他参数，重启会导致压测中断，指标数据不准确

## 参考资料

- [Golang 性能分析指南](https://github.com/xiaobaiTech/golangFamily/blob/main/README.md) - 更多 Go 性能分析技巧和方法
- [Go 内存模型和 GC 机制](https://golang.org/doc/gc-guide)