# SRE 工具集

该项目包含一系列面向SRE（站点可靠性工程）的工具和实用程序，旨在提高服务可靠性和性能。

## 项目结构

- **gogctuner**: Go垃圾回收调优工具，基于Uber的调优策略，动态优化GOGC，降低GC对CPU的影响

## 模块简介

### GOGCTuner

自动调整Go程序的GOGC（垃圾回收百分比）以平衡CPU和内存使用。

- **主要功能**：动态调整GOGC、容器感知、OOM保护、低开销监控
- **目标场景**：微服务、容器化环境、高CPU占用服务
- **详细文档**：[GOGCTuner说明](gogctuner/README.md)
- **优化策略**：[优化策略文档](gogctuner/优化策略.md)
- **实验案例**：[测试案例](gogctuner/example/README.md)

## 环境要求

- Go 1.14+
- Linux/macOS（Windows未完全测试）
- 容器化环境（Kubernetes、Docker）推荐

## 使用方法

每个工具都有独立的README和使用说明，请参考相应目录下的文档。

## 开发计划

- [ ] 为其他服务添加常见SRE指标采集器
- [ ] 系统资源利用率分析工具
- [ ] 服务依赖分析与可视化

## 贡献

欢迎提交PR或Issue来完善项目。

## 许可

MIT
