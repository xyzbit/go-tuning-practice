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

在以下三种负载模式下分别测试：
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

### 统一的测试用例及期望效果

#### 基准测试
```bash
./gogc_test -obj-size=4096 -load-type=constant
```
- **期望效果**：
  - GC频率：每分钟约 8-12 次
  - GC暂停时间：中位数约 1-3ms，p99 约 5-10ms
  - 堆内存使用：稳定在 100-150MB 左右
  - CPU使用：稳定在 5-15% 左右
  - 特点：GC行为相对稳定，但频率较高

#### GOGC调优测试
```bash
./gogc_test -obj-size=4096 -gogc=200 -load-type=constant
```
- **期望效果**：
  - GC频率：每分钟约 4-6 次（较基准测试减少约 50%）
  - GC暂停时间：中位数约 1.5-4ms，p99 约 8-15ms（较基准测试增加约 30-50%）
  - 堆内存使用：波动范围增大，峰值约 200-250MB
  - CPU使用：略有下降，约 4-12%
  - 特点：通过增加GOGC值，减少GC频率但增加单次GC耗时

#### Ballast技术测试
```bash
./gogc_test -obj-size=4096 -gogc=200 -ballast=100 -load-type=constant
```
- **期望效果**：
  - GC频率：每分钟约 2-4 次（较基准测试减少约 70%）
  - GC暂停时间：中位数约 2-5ms，p99 约 10-18ms
  - 堆内存使用：基准值提高约100MB，波动较小
  - CPU使用：较基准测试降低，约 3-10%
  - 特点：ballast提供了"假内存"基准，减少了GC触发频率，提高了内存利用率

#### 内存限制测试
```bash
./gogc_test -obj-size=4096 -gogc=200 -ballast=100 -memlimit=250 -load-type=constant
```
- **期望效果**：
  - GC频率：当接近内存限制时可能提高到每分钟 6-8 次
  - GC暂停时间：在接近内存限制时p99可能升高到 20-30ms
  - 堆内存使用：稳定在约 220-240MB，不会超过250MB
  - CPU使用：内存接近限制时可能升高到 15-25%
  - 特点：内存限制确保应用不会无限制使用内存，同时Go运行时会在接近限制时更积极回收

### 波动负载测试建议

对于波动负载测试，建议使用相同的参数配置，但将 `-load-type` 设置为 `wave`：

```bash
./gogc_test -obj-size=4096 -gogc=200 -ballast=100 -memlimit=250 -load-type=wave
```

- **期望效果**：
  - GC频率：随负载波动，在高峰期增加，低谷期减少
  - GC暂停时间：随堆内存大小波动，通常在负载增加后达到峰值
  - 堆内存使用：呈现正弦波形变化，与负载波动同步
  - 特点：观察GC如何适应动态负载变化，以及不同GOGC/内存策略如何影响系统弹性

### 尖刺负载测试建议

对于尖刺负载测试，建议使用相同的参数配置，但将 `-load-type` 设置为 `spike`：

```bash
./gogc_test -obj-size=4096 -gogc=200 -ballast=100 -memlimit=250 -load-type=spike
```

- **期望效果**：
  - GC频率：在尖刺期间急剧增加，可能达到每分钟 15-20 次
  - GC暂停时间：尖刺期间的p99可能达到 30-50ms
  - 堆内存使用：在尖刺期间迅速增加，尖刺后逐渐恢复
  - 特点：观察系统如何应对突发流量，尤其是不同GOGC值和内存策略对尖刺恢复的影响

### 测试方法

1. 对每个用例运行约5-10分钟，确保系统达到稳定状态
2. 使用Grafana观察关键指标变化：
   - `go_gc_duration_seconds` (p50, p99)
   - `go_gc_count` 的变化率
   - `go_memstats_heap_live_bytes` 的波动情况
   - `go_memstats_alloc_bytes` 和分配速率

3. 记录并比较不同配置下的表现差异，特别关注：
   - GC暂停对应用响应性的影响
   - 内存使用效率
   - CPU使用率
   - GC触发频率与业务负载的关系

### 高级测试场景

对于更深入的性能评估，还可以考虑以下测试场景：

1. **长期存活对象比例测试**：调整 `-long-lived` 参数（如0.2、0.5），观察不同世代对象比例对GC性能的影响

```bash
./gogc_test -obj-size=4096 -gogc=200 -long-lived=0.2 -load-type=constant
```

2. **对象大小变化测试**：测试不同大小对象（如 1KB、16KB）的影响

```bash
./gogc_test -obj-size=1024 -gogc=200 -load-type=constant
./gogc_test -obj-size=16384 -gogc=200 -load-type=constant
```

3. **极端GOGC值测试**：测试极低（50）或极高（500）GOGC值的影响

```bash
./gogc_test -obj-size=4096 -gogc=50 -load-type=constant
./gogc_test -obj-size=4096 -gogc=500 -load-type=constant
```
