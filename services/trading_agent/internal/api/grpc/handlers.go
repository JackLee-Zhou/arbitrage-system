// trading_agent/api/grpc — gRPC 请求处理器
package grpc

// TODO: 实现所有 gRPC 方法的业务处理逻辑
// GetSystemStatus / EmergencyClose / PauseStrategy / ResumeStrategy /
// UpdateRiskParams / SubscribeStateChanges
// 每个 handler 从 gRPC 请求提取参数 → 调 domain 层 → 返回 gRPC 响应
