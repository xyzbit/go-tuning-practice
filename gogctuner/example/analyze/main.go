package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 指标数据点
type DataPoint struct {
	Timestamp time.Time
	GOGC      int
	HeapMB    int
	Objects   int
	GCCount   int
	MemRatio  float64
	CPUTime   float64 // 新增：GC CPU耗时（毫秒）
}

func main() {
	logFile := flag.String("log", "", "测试日志文件路径")
	outputFile := flag.String("output", "report.txt", "输出报告文件路径")
	chartOutput := flag.String("chart", "chart.html", "图表输出文件路径")
	flag.Parse()

	if *logFile == "" {
		fmt.Println("请使用 -log 参数指定日志文件路径")
		fmt.Println("使用方法: go run analyze.go -log test_output.log [-output report.txt] [-chart chart.html]")
		os.Exit(1)
	}

	// 解析日志文件
	dataPoints, err := parseLogFile(*logFile)
	if err != nil {
		fmt.Printf("解析日志文件失败: %v\n", err)
		os.Exit(1)
	}

	if len(dataPoints) == 0 {
		fmt.Println("未找到有效的指标数据")
		os.Exit(1)
	}

	// 生成报告
	report := generateReport(dataPoints)

	// 保存报告
	err = os.WriteFile(*outputFile, []byte(report), 0o644)
	if err != nil {
		fmt.Printf("保存报告失败: %v\n", err)
		os.Exit(1)
	}

	// 生成时间线图表
	err = generateTimelineChart(dataPoints, *chartOutput)
	if err != nil {
		fmt.Printf("生成图表失败: %v\n", err)
	} else {
		fmt.Printf("图表已生成: %s\n", *chartOutput)
	}

	fmt.Printf("分析完成，报告已保存至 %s\n", *outputFile)
	// 打印摘要
	fmt.Println("\n====== 分析摘要 ======")
	fmt.Printf("数据点数量: %d\n", len(dataPoints))
	fmt.Printf("测试持续时间: %v\n", dataPoints[len(dataPoints)-1].Timestamp.Sub(dataPoints[0].Timestamp))
	fmt.Printf("GOGC范围: %d - %d\n", minGOGC(dataPoints), maxGOGC(dataPoints))
	fmt.Printf("内存使用率峰值: %.2f%%\n", maxMemRatio(dataPoints)*100)
	fmt.Printf("GOGC调整次数: %d\n", countGOGCChanges(dataPoints))
}

// 解析日志文件提取指标数据
func parseLogFile(filePath string) ([]DataPoint, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var dataPoints []DataPoint
	scanner := bufio.NewScanner(file)

	// 指标行匹配模式
	metricsRegex := regexp.MustCompile(`指标报告 - GOGC: (\d+), 堆内存: (\d+)MB, 对象数: (\d+), GC次数: (\d+), 内存使用率: ([\d\.]+)%`)
	// 新增：GC CPU耗时匹配模式
	gcTimeRegex := regexp.MustCompile(`GC耗时: ([\d\.]+)ms`)
	timeRegex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)

	for scanner.Scan() {
		line := scanner.Text()

		// 尝试提取时间戳
		var timestamp time.Time
		timeMatch := timeRegex.FindStringSubmatch(line)
		if len(timeMatch) > 1 {
			timestamp, _ = time.Parse("2006/01/02 15:04:05", timeMatch[1])
		} else {
			// 使用当前时间作为备选
			timestamp = time.Now()
		}

		// 尝试匹配指标行
		matches := metricsRegex.FindStringSubmatch(line)
		if len(matches) > 5 {
			gogc, _ := strconv.Atoi(matches[1])
			heapMB, _ := strconv.Atoi(matches[2])
			objects, _ := strconv.Atoi(matches[3])
			gcCount, _ := strconv.Atoi(matches[4])
			memRatio, _ := strconv.ParseFloat(matches[5], 64)

			// 尝试提取GC耗时
			cpuTime := 0.0
			gcTimeMatches := gcTimeRegex.FindStringSubmatch(line)
			if len(gcTimeMatches) > 1 {
				cpuTime, _ = strconv.ParseFloat(gcTimeMatches[1], 64)
			}

			dataPoints = append(dataPoints, DataPoint{
				Timestamp: timestamp,
				GOGC:      gogc,
				HeapMB:    heapMB,
				Objects:   objects,
				GCCount:   gcCount,
				MemRatio:  memRatio / 100, // 转换为小数
				CPUTime:   cpuTime,        // GC CPU耗时
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return dataPoints, nil
}

// 生成时间线图表
func generateTimelineChart(dataPoints []DataPoint, outputPath string) error {
	if len(dataPoints) == 0 {
		return fmt.Errorf("没有数据点")
	}

	// 准备数据
	var timeLabels, gogcValues, heapMBValues, objectsValues, gcCountValues, memRatioValues, cpuTimeValues []string
	var lastGCCount int

	// 获取初始时间
	startTime := dataPoints[0].Timestamp

	for i, dp := range dataPoints {
		// 计算相对时间（秒）
		relativeTime := dp.Timestamp.Sub(startTime).Seconds()
		timeLabel := fmt.Sprintf("%.1f", relativeTime)
		timeLabels = append(timeLabels, timeLabel)

		gogcValues = append(gogcValues, fmt.Sprintf("%d", dp.GOGC))
		heapMBValues = append(heapMBValues, fmt.Sprintf("%d", dp.HeapMB))
		objectsValues = append(objectsValues, fmt.Sprintf("%d", dp.Objects))

		// 计算增量GC次数
		var gcDelta int
		if i == 0 {
			gcDelta = 0
		} else {
			gcDelta = dp.GCCount - lastGCCount
		}
		lastGCCount = dp.GCCount
		gcCountValues = append(gcCountValues, fmt.Sprintf("%d", gcDelta))

		memRatioValues = append(memRatioValues, fmt.Sprintf("%.2f", dp.MemRatio*100))
		cpuTimeValues = append(cpuTimeValues, fmt.Sprintf("%.2f", dp.CPUTime))
	}

	// 生成HTML图表
	html := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>GOGCTuner性能分析图表</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        .chart-container {
            width: 90%;
            margin: 20px auto;
            height: 400px;
        }
        h1, h2 {
            text-align: center;
            font-family: Arial, sans-serif;
        }
    </style>
</head>
<body>
    <h1>GOGCTuner性能分析时间线</h1>
    
    <div class="chart-container">
        <h2>内存占用随时间变化</h2>
        <canvas id="memoryChart"></canvas>
    </div>
    
    <div class="chart-container">
        <h2>GC次数随时间变化</h2>
        <canvas id="gcCountChart"></canvas>
    </div>
    
    <div class="chart-container">
        <h2>CPU耗时随时间变化</h2>
        <canvas id="cpuTimeChart"></canvas>
    </div>
    
    <div class="chart-container">
        <h2>GOGC值随时间变化</h2>
        <canvas id="gogcChart"></canvas>
    </div>
    
    <script>
        // 公共配置
        const timeLabels = [` + strings.Join(timeLabels, ",") + `];
        
        // 内存图表
        new Chart(document.getElementById('memoryChart'), {
            type: 'line',
            data: {
                labels: timeLabels,
                datasets: [{
                    label: '堆内存 (MB)',
                    data: [` + strings.Join(heapMBValues, ",") + `],
                    borderColor: 'rgba(75, 192, 192, 1)',
                    backgroundColor: 'rgba(75, 192, 192, 0.2)',
                    tension: 0.1
                }, {
                    label: '内存使用率 (%)',
                    data: [` + strings.Join(memRatioValues, ",") + `],
                    borderColor: 'rgba(255, 99, 132, 1)',
                    backgroundColor: 'rgba(255, 99, 132, 0.2)',
                    tension: 0.1,
                    yAxisID: 'y1'
                }]
            },
            options: {
                responsive: true,
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: '时间 (秒)'
                        }
                    },
                    y: {
                        title: {
                            display: true,
                            text: '堆内存 (MB)'
                        }
                    },
                    y1: {
                        position: 'right',
                        title: {
                            display: true,
                            text: '内存使用率 (%)'
                        },
                        min: 0,
                        max: 100
                    }
                }
            }
        });
        
        // GC次数图表
        new Chart(document.getElementById('gcCountChart'), {
            type: 'bar',
            data: {
                labels: timeLabels,
                datasets: [{
                    label: 'GC次数增量',
                    data: [` + strings.Join(gcCountValues, ",") + `],
                    backgroundColor: 'rgba(153, 102, 255, 0.6)'
                }]
            },
            options: {
                responsive: true,
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: '时间 (秒)'
                        }
                    },
                    y: {
                        title: {
                            display: true,
                            text: 'GC次数'
                        },
                        beginAtZero: true
                    }
                }
            }
        });
        
        // CPU耗时图表
        new Chart(document.getElementById('cpuTimeChart'), {
            type: 'line',
            data: {
                labels: timeLabels,
                datasets: [{
                    label: 'GC CPU耗时 (ms)',
                    data: [` + strings.Join(cpuTimeValues, ",") + `],
                    borderColor: 'rgba(255, 159, 64, 1)',
                    backgroundColor: 'rgba(255, 159, 64, 0.2)',
                    tension: 0.1
                }]
            },
            options: {
                responsive: true,
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: '时间 (秒)'
                        }
                    },
                    y: {
                        title: {
                            display: true,
                            text: 'CPU耗时 (ms)'
                        },
                        beginAtZero: true
                    }
                }
            }
        });
        
        // GOGC图表
        new Chart(document.getElementById('gogcChart'), {
            type: 'line',
            data: {
                labels: timeLabels,
                datasets: [{
                    label: 'GOGC值',
                    data: [` + strings.Join(gogcValues, ",") + `],
                    borderColor: 'rgba(54, 162, 235, 1)',
                    backgroundColor: 'rgba(54, 162, 235, 0.2)',
                    tension: 0.1
                }]
            },
            options: {
                responsive: true,
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: '时间 (秒)'
                        }
                    },
                    y: {
                        title: {
                            display: true,
                            text: 'GOGC值'
                        },
                        beginAtZero: true
                    }
                }
            }
        });
    </script>
</body>
</html>
`

	return os.WriteFile(outputPath, []byte(html), 0o644)
}

// 生成分析报告
func generateReport(dataPoints []DataPoint) string {
	var report strings.Builder

	// 报告标题
	report.WriteString("# GOGCTuner 性能测试分析报告\n\n")
	report.WriteString(fmt.Sprintf("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	report.WriteString(fmt.Sprintf("测试持续时间: %v\n", dataPoints[len(dataPoints)-1].Timestamp.Sub(dataPoints[0].Timestamp)))
	report.WriteString(fmt.Sprintf("数据点数量: %d\n\n", len(dataPoints)))

	// GOGC分析
	report.WriteString("## GOGC调优分析\n\n")
	report.WriteString(fmt.Sprintf("最小GOGC: %d\n", minGOGC(dataPoints)))
	report.WriteString(fmt.Sprintf("最大GOGC: %d\n", maxGOGC(dataPoints)))
	report.WriteString(fmt.Sprintf("平均GOGC: %.1f\n", avgGOGC(dataPoints)))
	report.WriteString(fmt.Sprintf("GOGC调整次数: %d\n", countGOGCChanges(dataPoints)))
	report.WriteString(fmt.Sprintf("平均调整间隔: %.1f秒\n\n", avgGOGCChangeInterval(dataPoints).Seconds()))

	// 内存使用分析
	report.WriteString("## 内存使用分析\n\n")
	report.WriteString(fmt.Sprintf("最大堆内存: %dMB\n", maxHeapMB(dataPoints)))
	report.WriteString(fmt.Sprintf("平均堆内存: %dMB\n", avgHeapMB(dataPoints)))
	report.WriteString(fmt.Sprintf("最大内存使用率: %.2f%%\n", maxMemRatio(dataPoints)*100))
	report.WriteString(fmt.Sprintf("平均内存使用率: %.2f%%\n\n", avgMemRatio(dataPoints)*100))

	// GC活动分析
	report.WriteString("## GC活动分析\n\n")
	report.WriteString(fmt.Sprintf("GC总次数: %d\n", dataPoints[len(dataPoints)-1].GCCount-dataPoints[0].GCCount))
	report.WriteString(fmt.Sprintf("平均GC频率: 每%.1f秒一次\n", calcGCFrequency(dataPoints)))

	// 新增：GC CPU耗时分析
	if maxCPUTime(dataPoints) > 0 {
		report.WriteString(fmt.Sprintf("最大GC CPU耗时: %.2fms\n", maxCPUTime(dataPoints)))
		report.WriteString(fmt.Sprintf("平均GC CPU耗时: %.2fms\n\n", avgCPUTime(dataPoints)))
	} else {
		report.WriteString("日志中未包含GC CPU耗时数据\n\n")
	}

	// GOGC与内存关系
	report.WriteString("## GOGC与内存使用率关系\n\n")
	report.WriteString("内存使用率 -> 平均GOGC值:\n")

	memRatioBuckets := map[string][]int{
		"0-10%":   {},
		"10-20%":  {},
		"20-30%":  {},
		"30-40%":  {},
		"40-50%":  {},
		"50-60%":  {},
		"60-70%":  {},
		"70-80%":  {},
		"80-90%":  {},
		"90-100%": {},
	}

	for _, dp := range dataPoints {
		percent := dp.MemRatio * 100
		switch {
		case percent < 10:
			memRatioBuckets["0-10%"] = append(memRatioBuckets["0-10%"], dp.GOGC)
		case percent < 20:
			memRatioBuckets["10-20%"] = append(memRatioBuckets["10-20%"], dp.GOGC)
		case percent < 30:
			memRatioBuckets["20-30%"] = append(memRatioBuckets["20-30%"], dp.GOGC)
		case percent < 40:
			memRatioBuckets["30-40%"] = append(memRatioBuckets["30-40%"], dp.GOGC)
		case percent < 50:
			memRatioBuckets["40-50%"] = append(memRatioBuckets["40-50%"], dp.GOGC)
		case percent < 60:
			memRatioBuckets["50-60%"] = append(memRatioBuckets["50-60%"], dp.GOGC)
		case percent < 70:
			memRatioBuckets["60-70%"] = append(memRatioBuckets["60-70%"], dp.GOGC)
		case percent < 80:
			memRatioBuckets["70-80%"] = append(memRatioBuckets["70-80%"], dp.GOGC)
		case percent < 90:
			memRatioBuckets["80-90%"] = append(memRatioBuckets["80-90%"], dp.GOGC)
		default:
			memRatioBuckets["90-100%"] = append(memRatioBuckets["90-100%"], dp.GOGC)
		}
	}

	for bucket, values := range memRatioBuckets {
		if len(values) > 0 {
			var sum int
			for _, v := range values {
				sum += v
			}
			avg := float64(sum) / float64(len(values))
			report.WriteString(fmt.Sprintf("- 内存使用率 %s: 平均GOGC=%.1f (样本数=%d)\n", bucket, avg, len(values)))
		}
	}

	report.WriteString("\n## 结论与建议\n\n")

	// 根据数据给出结论和建议
	if maxMemRatio(dataPoints) > 0.9 {
		report.WriteString("- 内存使用率在测试期间接近上限，建议调低SafetyFactor或增加内存限制\n")
	}

	if countGOGCChanges(dataPoints) < 5 {
		report.WriteString("- GOGC调整频率较低，表明内存使用稳定或服务负载变化不大\n")
	} else {
		report.WriteString("- GOGC频繁调整，表明服务负载变化明显，GOGCTuner正在积极响应\n")
	}

	if maxGOGC(dataPoints) > 400 {
		report.WriteString("- GOGC最大值较高，可能导致单次GC耗时增加，建议设置合理的MaxGOGC上限\n")
	}

	if minGOGC(dataPoints) < 50 && maxMemRatio(dataPoints) > 0.7 {
		report.WriteString("- 内存使用率高且GOGC降至较低值，表明系统内存压力大，建议检查内存分配模式\n")
	}

	// 新增：GC CPU耗时分析结论
	if maxCPUTime(dataPoints) > 100 {
		report.WriteString("- GC CPU耗时峰值较高，可能导致应用程序暂停时间增加，建议优化内存分配模式或分配频率\n")
	}

	return report.String()
}

// 工具函数：计算最小GOGC
func minGOGC(dataPoints []DataPoint) int {
	if len(dataPoints) == 0 {
		return 0
	}
	min := dataPoints[0].GOGC
	for _, dp := range dataPoints {
		if dp.GOGC < min {
			min = dp.GOGC
		}
	}
	return min
}

// 工具函数：计算最大GOGC
func maxGOGC(dataPoints []DataPoint) int {
	if len(dataPoints) == 0 {
		return 0
	}
	max := dataPoints[0].GOGC
	for _, dp := range dataPoints {
		if dp.GOGC > max {
			max = dp.GOGC
		}
	}
	return max
}

// 工具函数：计算平均GOGC
func avgGOGC(dataPoints []DataPoint) float64 {
	if len(dataPoints) == 0 {
		return 0
	}
	var sum int
	for _, dp := range dataPoints {
		sum += dp.GOGC
	}
	return float64(sum) / float64(len(dataPoints))
}

// 工具函数：计算GOGC调整次数
func countGOGCChanges(dataPoints []DataPoint) int {
	if len(dataPoints) <= 1 {
		return 0
	}
	changes := 0
	for i := 1; i < len(dataPoints); i++ {
		if dataPoints[i].GOGC != dataPoints[i-1].GOGC {
			changes++
		}
	}
	return changes
}

// 工具函数：计算GOGC调整平均间隔
func avgGOGCChangeInterval(dataPoints []DataPoint) time.Duration {
	if len(dataPoints) <= 1 {
		return 0
	}

	var changePoints []time.Time
	for i := 1; i < len(dataPoints); i++ {
		if dataPoints[i].GOGC != dataPoints[i-1].GOGC {
			changePoints = append(changePoints, dataPoints[i].Timestamp)
		}
	}

	if len(changePoints) <= 1 {
		return 0
	}

	var totalInterval time.Duration
	for i := 1; i < len(changePoints); i++ {
		totalInterval += changePoints[i].Sub(changePoints[i-1])
	}

	return totalInterval / time.Duration(len(changePoints)-1)
}

// 工具函数：计算最大堆内存
func maxHeapMB(dataPoints []DataPoint) int {
	if len(dataPoints) == 0 {
		return 0
	}
	max := dataPoints[0].HeapMB
	for _, dp := range dataPoints {
		if dp.HeapMB > max {
			max = dp.HeapMB
		}
	}
	return max
}

// 工具函数：计算平均堆内存
func avgHeapMB(dataPoints []DataPoint) int {
	if len(dataPoints) == 0 {
		return 0
	}
	var sum int
	for _, dp := range dataPoints {
		sum += dp.HeapMB
	}
	return sum / len(dataPoints)
}

// 工具函数：计算最大内存使用率
func maxMemRatio(dataPoints []DataPoint) float64 {
	if len(dataPoints) == 0 {
		return 0
	}
	max := dataPoints[0].MemRatio
	for _, dp := range dataPoints {
		if dp.MemRatio > max {
			max = dp.MemRatio
		}
	}
	return max
}

// 工具函数：计算平均内存使用率
func avgMemRatio(dataPoints []DataPoint) float64 {
	if len(dataPoints) == 0 {
		return 0
	}
	var sum float64
	for _, dp := range dataPoints {
		sum += dp.MemRatio
	}
	return sum / float64(len(dataPoints))
}

// 工具函数：计算GC频率(秒/次)
func calcGCFrequency(dataPoints []DataPoint) float64 {
	if len(dataPoints) <= 1 {
		return 0
	}

	firstGC := dataPoints[0].GCCount
	lastGC := dataPoints[len(dataPoints)-1].GCCount

	if lastGC <= firstGC {
		return 0
	}

	durationSec := dataPoints[len(dataPoints)-1].Timestamp.Sub(dataPoints[0].Timestamp).Seconds()
	gcCount := lastGC - firstGC

	return durationSec / float64(gcCount)
}

// 新增：计算最大GC CPU耗时
func maxCPUTime(dataPoints []DataPoint) float64 {
	if len(dataPoints) == 0 {
		return 0
	}
	max := dataPoints[0].CPUTime
	for _, dp := range dataPoints {
		if dp.CPUTime > max {
			max = dp.CPUTime
		}
	}
	return max
}

// 新增：计算平均GC CPU耗时
func avgCPUTime(dataPoints []DataPoint) float64 {
	if len(dataPoints) == 0 {
		return 0
	}
	var sum float64
	var count int
	for _, dp := range dataPoints {
		if dp.CPUTime > 0 {
			sum += dp.CPUTime
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}
