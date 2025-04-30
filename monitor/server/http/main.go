package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/xyzbit/go-tuning-practice/monitor/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"google.golang.org/grpc"
	"mosn.io/holmes"
)

func initTracer() (*tracesdk.TracerProvider, error) {
	ctx := context.Background()
	exp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("my-service"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

// 模拟一个耗时的业务逻辑
func slowHandler(w http.ResponseWriter, r *http.Request) {
	_, span := otel.Tracer("my-service").Start(r.Context(), "slowHandler")
	defer span.End()

	time.Sleep(2 * time.Second)
	span.SetAttributes(attribute.String("result", "completed"))
	fmt.Fprintf(w, "Slow operation completed")
}

// 模拟一个快速的业务逻辑
func fastHandler(w http.ResponseWriter, r *http.Request) {
	_, span := otel.Tracer("my-service").Start(r.Context(), "fastHandler")
	defer span.End()

	span.SetAttributes(attribute.String("result", "completed"))
	fmt.Fprintf(w, "Fast operation completed")
}

// 模拟一个内存分配的函数
func allocHandler(wr http.ResponseWriter, req *http.Request) {
	_, span := otel.Tracer("my-service").Start(req.Context(), "allocHandler")
	defer span.End()

	var m = make(map[string]string, 1073741824)
	for i := 0; i < 1000; i++ {
		m[fmt.Sprint(i)] = fmt.Sprint(i)
	}
	span.SetAttributes(attribute.String("result", "completed"))
	_ = m
}

// 模拟一个1GB的slice
func make1gbslice(wr http.ResponseWriter, req *http.Request) {
	_, span := otel.Tracer("my-service").Start(req.Context(), "make1gbslice")
	defer span.End()

	var a = make([]byte, 1073741824)
	_ = a
	span.SetAttributes(attribute.String("result", "completed"))
}

// 模拟一个内存泄漏的函数
func leak(wr http.ResponseWriter, req *http.Request) {
	_, span := otel.Tracer("my-service").Start(req.Context(), "leak")
	defer span.End()

	span.SetAttributes(attribute.String("result", "trigger"))

	taskChan := make(chan int)
	consumer := func() {
		for task := range taskChan {
			_ = task // do some tasks
		}
	}

	producer := func() {
		for i := 0; i < 10; i++ {
			taskChan <- i // generate some tasks
		}
		// forget to close the taskChan here
	}

	go consumer()
	go producer()
}

// 请求rpc
func requestRpc() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewYourServiceClient(conn)

	// 调用 RPC 方法
	response, err := client.YourMethod(context.Background(), &pb.YourRequest{
	
}

func main() {
	h, err := holmes.New(
		holmes.WithCollectInterval("10s"),
		holmes.WithDumpToLogger(true),
		holmes.WithDumpPath("./holmes.log"),
		holmes.WithCPUDump(20, 10, 70, time.Minute),
		holmes.WithMemDump(20, 10, 70, time.Minute),
	)
	h.EnableCPUDump().EnableMemDump()

	if err != nil {
		panic(err)
	}
	h.Start()
	defer h.Stop()

	// 初始化追踪器
	tp, err := middleware.InitTracer(middleware.TracerConfig{
		ServiceName:    "your-service",
		ServiceVersion: "v1.0.0",
		Environment:    "development",
		OtlpEndpoint:   "localhost:4317",
	})
	if err != nil {
		panic(err)
	}
	defer tp.Shutdown(context.Background())

	// 创建路由
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	mux.HandleFunc("/slow", slowHandler)
	mux.HandleFunc("/fast", fastHandler)
	mux.HandleFunc("/alloc", allocHandler)
	mux.HandleFunc("/make1gbslice", make1gbslice)
	mux.HandleFunc("/leak", leak)
	handler := middleware.HTTPMiddleware(mux)

	handler = otelhttp.NewHandler(handler, "my-http-service")

	// 启动服务器
	fmt.Println("Server started at :8080")
	http.ListenAndServe(":8080", handler)
}
