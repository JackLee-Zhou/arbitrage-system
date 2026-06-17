// trading_agent/api/grpc — gRPC :9090（暴露给 orchestrator）
//
// 实现 orchestrator.proto 中定义的 OrchestratorService 服务。
// 注册 "raw" codec 实现 JSON 透传，与 orchestrator 的 rawCodec 兼容。
// 等 proto stub 生成后，切换到标准 protobuf codec。
package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

// ============================================================
// rawCodec — JSON 透传 codec，与 orchestrator 客户端 rawCodec 兼容
// 注册名 "raw"，使用 encoding/json 进行序列化/反序列化
// ============================================================

type rawCodec struct{}

func (rawCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (rawCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (rawCodec) Name() string { return "raw" }

func init() {
	encoding.RegisterCodec(rawCodec{})
}

// ============================================================
// ServiceDesc — gRPC 服务描述符
// 匹配 orchestrator.proto 中定义的 OrchestratorService
// proto stub 生成后，切换为 generated 包的 *_grpc.pb.go 注册
// ============================================================

// serviceName 必须与 proto 定义的 package + service 一致
const serviceName = "arbitrage.v1.OrchestratorService"

// TradingAgentServiceDesc 是 OrchestratorService 的 gRPC 服务描述符
// 等 proto 生成后，删除此描述符，改为：
//
//	grpcServer := grpc.NewServer()
//	arbitragev1.RegisterOrchestratorServiceServer(grpcServer, handler)
var TradingAgentServiceDesc = &grpc.ServiceDesc{
	ServiceName: serviceName,
	HandlerType: (*TradingAgentServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetStatus",
			Handler:    _TradingAgent_GetStatus_Handler,
		},
		{
			MethodName: "GetPositions",
			Handler:    _TradingAgent_GetPositions_Handler,
		},
		{
			MethodName: "EmergencyClose",
			Handler:    _TradingAgent_EmergencyClose_Handler,
		},
		{
			MethodName: "PauseStrategy",
			Handler:    _TradingAgent_PauseStrategy_Handler,
		},
		{
			MethodName: "ResumeStrategy",
			Handler:    _TradingAgent_ResumeStrategy_Handler,
		},
		{
			MethodName: "UpdateRiskParams",
			Handler:    _TradingAgent_UpdateRiskParams_Handler,
		},
		{
			MethodName: "HealthCheck",
			Handler:    _TradingAgent_HealthCheck_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "orchestrator.proto",
}

// ============================================================
// gRPC 方法 Handler（对应 grpc.MethodDesc.Handler 签名）
// ============================================================

func _TradingAgent_GetStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	in := &StatusRequest{}
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(TradingAgentServer).GetStatus(ctx, in)
}

func _TradingAgent_GetPositions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	in := &PositionRequest{}
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(TradingAgentServer).GetPositions(ctx, in)
}

func _TradingAgent_EmergencyClose_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	in := &EmergencyCloseRequest{}
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(TradingAgentServer).EmergencyClose(ctx, in)
}

func _TradingAgent_PauseStrategy_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	in := &PauseStrategyRequest{}
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(TradingAgentServer).PauseStrategy(ctx, in)
}

func _TradingAgent_ResumeStrategy_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	in := &ResumeStrategyRequest{}
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(TradingAgentServer).ResumeStrategy(ctx, in)
}

func _TradingAgent_UpdateRiskParams_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	in := &UpdateRiskRequest{}
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(TradingAgentServer).UpdateRiskParams(ctx, in)
}

func _TradingAgent_HealthCheck_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
	in := &HealthRequest{}
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(TradingAgentServer).HealthCheck(ctx, in)
}

// ============================================================
// GRPCServer — gRPC 服务封装
// ============================================================

// GRPCServer gRPC 服务端点封装
type GRPCServer struct {
	lis     net.Listener
	server  *grpc.Server
	addr    string
	started atomic.Bool
}

// NewGRPCServer 创建 gRPC 服务
// handler 必须实现 TradingAgentServer 接口
func NewGRPCServer(addr string, handler *TradingAgentHandler) *GRPCServer {
	srv := grpc.NewServer()
	srv.RegisterService(TradingAgentServiceDesc, handler)
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
