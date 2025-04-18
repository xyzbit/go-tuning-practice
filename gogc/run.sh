#!/bin/bash

# 显示帮助信息
show_help() {
    echo "用法: $0 [选项]"
    echo "选项:"
    echo "  -h, --help               显示帮助信息"
    echo "  -b, --build              仅构建应用"
    echo "  -r, --run                仅运行应用"
    echo "  -p, --prometheus         启动 Prometheus 和 Grafana"
    echo "  -s, --stop-prometheus    停止 Prometheus 和 Grafana"
    echo "  -g, --gogc VALUE         设置 GOGC 值 (默认: 100)"
    echo "  -o, --obj-size VALUE     设置对象大小 (默认: 1024 字节)"
    echo "  -a, --alloc-rate VALUE   设置分配速率 (默认: 1000 对象/秒)"
    echo "  -l, --long-lived VALUE   设置长期存活比例 (默认: 0.05)"
    echo "  -t, --load-type TYPE     设置负载类型: constant, wave, spike (默认: constant)"
    echo "  --port VALUE             设置 HTTP 端口 (默认: 8080)"
    echo ""
    echo "示例:"
    echo "  $0 -b -r -g 200                # 构建并运行，GOGC 设置为 200"
    echo "  $0 -p -r -o 2048 -a 2000       # 启动 Prometheus 并运行，对象大小 2048 字节，分配速率 2000/秒"
    echo "  $0 -r -t wave                  # 运行波动负载模式"
    echo "  $0 -r -t spike -o 4096         # 运行尖刺负载模式，对象大小 4096 字节"
}

# 默认值
BUILD=false
RUN=false
START_PROMETHEUS=false
STOP_PROMETHEUS=false
GOGC=100
OBJ_SIZE=1024
ALLOC_RATE=1000
LONG_LIVED=0.05
PORT=8080
LOAD_TYPE="constant"

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -b|--build)
            BUILD=true
            shift
            ;;
        -r|--run)
            RUN=true
            shift
            ;;
        -p|--prometheus)
            START_PROMETHEUS=true
            shift
            ;;
        -s|--stop-prometheus)
            STOP_PROMETHEUS=true
            shift
            ;;
        -g|--gogc)
            GOGC="$2"
            shift 2
            ;;
        -o|--obj-size)
            OBJ_SIZE="$2"
            shift 2
            ;;
        -a|--alloc-rate)
            ALLOC_RATE="$2"
            shift 2
            ;;
        -l|--long-lived)
            LONG_LIVED="$2"
            shift 2
            ;;
        -t|--load-type)
            LOAD_TYPE="$2"
            shift 2
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        *)
            echo "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 如果没有指定操作，显示帮助
if [[ $BUILD == false && $RUN == false && $START_PROMETHEUS == false && $STOP_PROMETHEUS == false ]]; then
    show_help
    exit 0
fi

# 构建应用
if [[ $BUILD == true ]]; then
    echo "正在构建应用..."
    go build -o gogc_test
    echo "构建完成。"
fi

# 启动 Prometheus
if [[ $START_PROMETHEUS == true ]]; then
    echo "正在启动 Prometheus 和 Grafana..."
    docker-compose up -d
    echo "Prometheus 已启动，访问 http://localhost:9090"
    echo "Grafana 已启动，访问 http://localhost:3000 (用户名/密码: admin/admin)"
fi

# 停止 Prometheus
if [[ $STOP_PROMETHEUS == true ]]; then
    echo "正在停止 Prometheus 和 Grafana..."
    docker-compose down
    echo "服务已停止。"
fi

# 运行应用
if [[ $RUN == true ]]; then
    echo "正在运行应用..."
    echo "GOGC=$GOGC, 对象大小=$OBJ_SIZE, 分配速率=$ALLOC_RATE, 长期存活比例=$LONG_LIVED, 负载类型=$LOAD_TYPE, 端口=$PORT"
    
    if [[ ! -f ./gogc_test ]]; then
        echo "应用未构建，正在构建..."
        go build -o gogc_test
    fi
    
    ./gogc_test -gogc=$GOGC -obj-size=$OBJ_SIZE -alloc-rate=$ALLOC_RATE -long-lived=$LONG_LIVED -port=$PORT -load-type=$LOAD_TYPE
fi 