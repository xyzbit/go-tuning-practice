package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
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

func main() {
	h, err := holmes.New(
		holmes.WithCollectInterval("10s"),
		holmes.WithCPUDump(80, 80, 80, time.Second*10),
		holmes.WithMemDump(80, 80, 80, time.Second*10),
	)
	if err != nil {
		panic(err)
	}
	if err := h.Start(); err != nil {
		panic(err)
	}
	defer h.Stop()

	tp, err := initTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	http.HandleFunc("/slow", slowHandler)
	http.HandleFunc("/fast", fastHandler)

	handler := otelhttp.NewHandler(http.DefaultServeMux, "my-service")
	fmt.Println("Server started at :8080")
	http.ListenAndServe(":8080", handler)
}
