package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/xyzbit/go-tuning-practice/monitor/server/grpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HelloServer 实现 HelloService 接口
type HelloServer struct {
	pb.UnimplementedHelloServiceServer
	serverName string
}

// NewHelloServer 创建一个新的 HelloServer
func NewHelloServer(name string) *HelloServer {
	return &HelloServer{
		serverName: name,
	}
}

// Hello 实现一元 RPC 方法
func (s *HelloServer) Hello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	greeting := fmt.Sprintf("Hello, %s! %s", req.Name, req.Message)
	return &pb.HelloResponse{
		Greeting:   greeting,
		Timestamp:  time.Now().Unix(),
		ServerName: s.serverName,
		Status:     pb.Status_SUCCESS,
	}, nil
}

// HelloStream 实现服务端流式 RPC 方法
func (s *HelloServer) HelloStream(req *pb.HelloRequest, stream pb.HelloService_HelloStreamServer) error {
	if req.Name == "" {
		return status.Error(codes.InvalidArgument, "name is required")
	}

	for i := 0; i < 5; i++ {
		greeting := fmt.Sprintf("Hello %s! Message %d: %s", req.Name, i+1, req.Message)
		if err := stream.Send(&pb.HelloResponse{
			Greeting:   greeting,
			Timestamp:  time.Now().Unix(),
			ServerName: s.serverName,
			Status:     pb.Status_SUCCESS,
		}); err != nil {
			return err
		}
		time.Sleep(time.Second) // 模拟处理时间
	}
	return nil
}

// HelloClientStream 实现客户端流式 RPC 方法
func (s *HelloServer) HelloClientStream(stream pb.HelloService_HelloClientStreamServer) error {
	var messages []string

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// 客户端已完成发送
			greeting := fmt.Sprintf("Received all messages: %v", messages)
			return stream.SendAndClose(&pb.HelloResponse{
				Greeting:   greeting,
				Timestamp:  time.Now().Unix(),
				ServerName: s.serverName,
				Status:     pb.Status_SUCCESS,
			})
		}
		if err != nil {
			return err
		}
		messages = append(messages, fmt.Sprintf("%s: %s", req.Name, req.Message))
	}
}

// HelloBiStream 实现双向流式 RPC 方法
func (s *HelloServer) HelloBiStream(stream pb.HelloService_HelloBiStreamServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		greeting := fmt.Sprintf("Hello %s! Received: %s", req.Name, req.Message)
		if err := stream.Send(&pb.HelloResponse{
			Greeting:   greeting,
			Timestamp:  time.Now().Unix(),
			ServerName: s.serverName,
			Status:     pb.Status_SUCCESS,
		}); err != nil {
			return err
		}
	}
}
