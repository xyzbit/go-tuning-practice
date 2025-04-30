package middleware

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MetadataTextMapCarrier 实现 TextMapCarrier 接口
type MetadataTextMapCarrier metadata.MD

func (m MetadataTextMapCarrier) Get(key string) string {
	values := metadata.MD(m).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (m MetadataTextMapCarrier) Set(key string, value string) {
	metadata.MD(m).Set(key, value)
}

func (m MetadataTextMapCarrier) Keys() []string {
	out := make([]string, 0, len(m))
	for key := range metadata.MD(m) {
		out = append(out, key)
	}
	return out
}

// TracerConfig 追踪配置
type TracerConfig struct {
	ServiceName    string // 服务名称
	ServiceVersion string // 服务版本
	Environment    string // 环境（如：production, staging, development）
	OtlpEndpoint   string // OTLP endpoint
}

// InitTracer 初始化追踪器
func InitTracer(cfg TracerConfig) (*tracesdk.TracerProvider, error) {
	ctx := context.Background()

	// 创建 OTLP exporter
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OtlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 OTLP exporter 失败: %w", err)
	}

	// 创建资源属性
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(cfg.ServiceName),
		semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		attribute.String("environment", cfg.Environment),
	)

	// 创建 TracerProvider
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(res),
	)

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tp)

	// 设置全局 TextMapPropagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// HTTPMiddleware HTTP中间件
func HTTPMiddleware(next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, "http-server")
}

// GRPCUnaryServerInterceptor gRPC一元拦截器
func GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		tracer := otel.Tracer("grpc-server")
		name := info.FullMethod

		var span trace.Span
		ctx, span = tracer.Start(ctx, name)
		defer span.End()

		// 从metadata中提取追踪信息
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			carrier := MetadataTextMapCarrier(md)
			otel.GetTextMapPropagator().Extract(ctx, carrier)
		}

		// 添加RPC属性
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
		)

		return handler(ctx, req)
	}
}

// GRPCStreamServerInterceptor gRPC流式拦截器
func GRPCStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		tracer := otel.Tracer("grpc-server")
		name := info.FullMethod

		ctx := ss.Context()
		var span trace.Span
		ctx, span = tracer.Start(ctx, name)
		defer span.End()

		// 从metadata中提取追踪信息
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			carrier := MetadataTextMapCarrier(md)
			otel.GetTextMapPropagator().Extract(ctx, carrier)
		}

		// 添加RPC属性
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
			attribute.Bool("rpc.stream", true),
		)

		// 包装 ServerStream 以传递上下文
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// wrappedServerStream 包装 grpc.ServerStream 以支持上下文传递
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
