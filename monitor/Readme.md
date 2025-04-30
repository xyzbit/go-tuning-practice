## 监控模块

监控是对程序运行状态的可视化，目的是发现运行中的问题，其中最常用的几种方式
- OpenTelemetry 全链路追踪，适合复杂的微服务业务，清晰洞察时间花在哪
- holmes 运行时性能数据查看+dump，用于性能优化分析


通过 holmes profile(具体到代码行的开销) + OpenTelemetry 各个服务的耗时