package main

import (
	"context"
	"net"

	"github.com/xyzbit/go-tuning-practice/monitor/middleware"
	"google.golang.org/grpc"
)

func main() {
	// 初始化追踪器
	tp, err := middleware.InitTracer(middleware.TracerConfig{
		ServiceName:    "your-grpc-service",
		ServiceVersion: "v1.0.0",
		Environment:    "development",
		OtlpEndpoint:   "localhost:4317",
	})
	if err != nil {
		panic(err)
	}
	defer tp.Shutdown(context.Background())

	// 创建 gRPC 服务器
	server := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.GRPCUnaryServerInterceptor()),
		grpc.StreamInterceptor(middleware.GRPCStreamServerInterceptor()),
	)

	// 注册你的 gRPC 服务
	pb.RegisterYourServiceServer(server, &YourService{})

	// 启动服务器
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		panic(err)
	}
	server.Serve(lis)
}

// 你的 gRPC 服务实现
type YourService struct {
	pb.UnimplementedYourServiceServer
}

func (s *YourService) Hello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	return &pb.HelloResponse{Message: "Hello, World!"}, nil
}
