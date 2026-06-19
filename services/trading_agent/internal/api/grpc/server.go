// trading_agent/api/grpc — gRPC :9090（暴露给 orchestrator）
//
// 使用 protoc 生成的 arbiragev1 包中的 OrchestratorService gRPC stub。
package grpc

import (
	"fmt"
	"log"
	"net"
	"sync/atomic"

	"google.golang.org/grpc"

	"github.com/arbitrage-system/contracts/generated/go/arbitragev1"
)

// GRPCServer gRPC 服务端点封装
type GRPCServer struct {
	lis     net.Listener
	server  *grpc.Server
	addr    string
	started atomic.Bool
}

// NewGRPCServer 创建 gRPC 服务，注册 OrchestratorService
func NewGRPCServer(addr string, handler arbitragev1.OrchestratorServiceServer) *GRPCServer {
	srv := grpc.NewServer()
	arbitragev1.RegisterOrchestratorServiceServer(srv, handler)
	return &GRPCServer{
		addr:   addr,
		server: srv,
	}
}

// Start 启动 gRPC 服务（非阻塞）
func (s *GRPCServer) Start() error {
	if s.started.Load() {
		return fmt.Errorf("grpc server already started on %s", s.addr)
	}

	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpc listen %s: %w", s.addr, err)
	}
	s.lis = lis
	s.started.Store(true)

	log.Printf("[grpc] serving on %s", s.addr)
	go func() {
		if err := s.server.Serve(lis); err != nil {
			log.Printf("[grpc] serve error: %v", err)
		}
	}()
	return nil
}

// Stop 优雅关闭 gRPC 服务（阻塞等待所有 RPC 完成）
func (s *GRPCServer) Stop() {
	if !s.started.Load() {
		return
	}
	log.Println("[grpc] graceful stop...")
	s.server.GracefulStop()
	s.started.Store(false)
	if s.lis != nil {
		_ = s.lis.Close()
	}
}
