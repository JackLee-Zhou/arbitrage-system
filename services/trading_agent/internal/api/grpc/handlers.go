// trading_agent/api/grpc — gRPC 请求处理器
//
// 实现 orchestrator.proto 中定义的 OrchestratorService 7 个 RPC 方法。
// 当前使用 Go 原生结构体 + JSON 序列化（rawCodec），等 proto stub 生成后切换。
package grpc

import (
	"context"
	"fmt"
	"time"
)

// ============================================================
// 消息类型（匹配 contracts/orchestrator.proto 定义）
// proto 生成 Go stub 后，切换到 generated 包中的类型
// ============================================================

// StatusRequest 状态查询请求
type StatusRequest struct {
	RequestID string `json:"request_id,omitempty"`
}

// StatusResponse 状态查询响应
type StatusResponse struct {
	EngineStatus   string   `json:"engine_status"`      // "running" / "paused" / "safe_mode"
	LastTickTime   int64    `json:"last_tick_time"`     // unix timestamp
	LastSignalTime int64    `json:"last_signal_time"`   // unix timestamp
	CurrentAIScore float64  `json:"current_ai_score"`   // 当前 AI 评分
	AIScoreDecay   float64  `json:"ai_score_decay"`     // AI 评分老化系数
	OpenOrders     int32    `json:"open_orders"`        // 未成交报单数
	LogTail        []string `json:"log_tail,omitempty"` // 最近日志（可选）
}

// PositionRequest 持仓查询请求（空）
type PositionRequest struct{}

// Position 持仓信息（匹配 contracts/position.proto）
type Position struct {
	Symbol      string  `json:"symbol"`
	Direction   string  `json:"direction"`    // "long" / "short"
	Volume      int32   `json:"volume"`       // 持仓手数
	AvgPrice    float64 `json:"avg_price"`    // 开仓均价
	LastPrice   float64 `json:"last_price"`   // 最新价
	FloatPnl    float64 `json:"float_pnl"`    // 浮动盈亏
	TodayVolume int32   `json:"today_volume"` // 今仓
	YdVolume    int32   `json:"yd_volume"`    // 昨仓
}

// PositionList 持仓列表响应
type PositionList struct {
	Positions []Position `json:"positions"`
	Timestamp int64      `json:"timestamp"` // unix timestamp
}

// EmergencyCloseRequest 紧急平仓请求
type EmergencyCloseRequest struct {
	Reason    string `json:"reason,omitempty"`     // 紧急平仓原因
	Symbol    string `json:"symbol,omitempty"`     // "MA2509" 或空 = 全部
	RequestBy string `json:"request_by,omitempty"` // 操作人
}

// EmergencyCloseResponse 紧急平仓响应
type EmergencyCloseResponse struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message,omitempty"`
	OrderIDs []string `json:"order_ids,omitempty"`
}

// PauseStrategyRequest 暂停策略请求
type PauseStrategyRequest struct {
	StrategyName string `json:"strategy_name,omitempty"`
	RequestBy    string `json:"request_by,omitempty"`
}

// PauseStrategyResponse 暂停策略响应
type PauseStrategyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ResumeStrategyRequest 恢复策略请求
type ResumeStrategyRequest struct {
	StrategyName string `json:"strategy_name,omitempty"`
	RequestBy    string `json:"request_by,omitempty"`
}

// ResumeStrategyResponse 恢复策略响应
type ResumeStrategyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// UpdateRiskRequest 修改风控参数请求
type UpdateRiskRequest struct {
	RiskParam string  `json:"risk_param"`           // 参数名
	NewValue  float64 `json:"new_value"`            // 新值
	RequestBy string  `json:"request_by,omitempty"` // 操作人
}

// UpdateRiskResponse 修改风控参数响应
type UpdateRiskResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// HealthRequest 健康检查请求（空）
type HealthRequest struct{}

// HealthCheckResponse gRPC 健康检查响应（匹配 contracts/alert.proto）
type HealthCheckResponse struct {
	Service string             `json:"service"`
	Status  string             `json:"status"` // "healthy" / "degraded" / "unhealthy"
	Version string             `json:"version"`
	Uptime  int64              `json:"uptime"` // 运行秒数
	Deps    []DependencyStatus `json:"deps,omitempty"`
}

// DependencyStatus 依赖状态（匹配 contracts/alert.proto）
type DependencyStatus struct {
	Name   string `json:"name"`   // "redis" / "influxdb" / "ctp"
	Status string `json:"status"` // "ok" / "error"
	Detail string `json:"detail,omitempty"`
}

// ============================================================
// 接口定义（依赖注入，避免循环依赖）
// 由 app/wire.go 装配层注入具体实现
// ============================================================

// EngineAccessor 引擎状态访问接口
type EngineAccessor interface {
	// GetStatus 返回引擎当前状态字符串
	GetStatus() string
	// GetLastTickTime 返回最后收到的 Tick 时间（unix timestamp）
	GetLastTickTime() int64
	// GetLastSignalTime 返回最后信号时间（unix timestamp）
	GetLastSignalTime() int64
	// GetCurrentAIScore 返回当前 AI 评分
	GetCurrentAIScore() float64
	// GetAIScoreDecay 返回 AI 评分老化系数
	GetAIScoreDecay() float64
	// GetOpenOrders 返回未成交报单数
	GetOpenOrders() int32
	// PauseStrategy 暂停指定策略
	PauseStrategy(name string) error
	// ResumeStrategy 恢复指定策略
	ResumeStrategy(name string) error
	// EnterSafeMode 进入安全模式
	EnterSafeMode(reason string) error
	// EmergencyClosePositions 执行实际平仓操作
	EmergencyClosePositions() error
}

// PositionAccessor 持仓访问接口
type PositionAccessor interface {
	// GetPositions 返回所有当前持仓
	GetPositions() []PositionInfo
}

// PositionInfo 持仓信息（API 层视图，与 domain/position 解耦）
type PositionInfo struct {
	Symbol      string
	Direction   string
	Volume      int32
	AvgPrice    float64
	LastPrice   float64
	FloatPnl    float64
	TodayVolume int32
	YdVolume    int32
}

// RiskAccessor 风控参数访问接口
type RiskAccessor interface {
	// UpdateRiskParams 修改风控参数
	UpdateRiskParams(param string, value float64) error
}

// ============================================================
// TradingAgentServer — gRPC 服务端接口
// 等 proto stub 生成后，改为生成的接口（如 arbitrage.v1.OrchestratorServiceServer）
// ============================================================

// TradingAgentServer 是 OrchestratorService 的服务端接口
type TradingAgentServer interface {
	GetStatus(ctx context.Context, req *StatusRequest) (*StatusResponse, error)
	GetPositions(ctx context.Context, req *PositionRequest) (*PositionList, error)
	EmergencyClose(ctx context.Context, req *EmergencyCloseRequest) (*EmergencyCloseResponse, error)
	PauseStrategy(ctx context.Context, req *PauseStrategyRequest) (*PauseStrategyResponse, error)
	ResumeStrategy(ctx context.Context, req *ResumeStrategyRequest) (*ResumeStrategyResponse, error)
	UpdateRiskParams(ctx context.Context, req *UpdateRiskRequest) (*UpdateRiskResponse, error)
	HealthCheck(ctx context.Context, req *HealthRequest) (*HealthCheckResponse, error)
	// SubscribeStateChanges — TODO: 需要 proto server stream 生成后才能实现
	// rpc SubscribeStateChanges(stream.SubscribeRequest) returns (stream.StateChange)
	// 等 proto 定义完成 + stream stub 生成后，在此接口追加方法
}

// ============================================================
// TradingAgentHandler — gRPC 方法实现
// ============================================================

// TradingAgentHandler 实现 TradingAgentServer 接口
// 通过 EngineAccessor / PositionAccessor / RiskAccessor 接口操作业务层
type TradingAgentHandler struct {
	engine      EngineAccessor
	positionMgr PositionAccessor
	riskMgr     RiskAccessor
	serviceName string
	version     string
	startTime   time.Time
}

// NewTradingAgentHandler 创建 gRPC handler
func NewTradingAgentHandler(
	engine EngineAccessor,
	positionMgr PositionAccessor,
	riskMgr RiskAccessor,
	serviceName string,
	version string,
) *TradingAgentHandler {
	return &TradingAgentHandler{
		engine:      engine,
		positionMgr: positionMgr,
		riskMgr:     riskMgr,
		serviceName: serviceName,
		version:     version,
		startTime:   time.Now(),
	}
}

// ============================================================
// RPC 方法实现
// ============================================================

// GetStatus 查询引擎状态
func (h *TradingAgentHandler) GetStatus(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	return &StatusResponse{
		EngineStatus:   h.engine.GetStatus(),
		LastTickTime:   h.engine.GetLastTickTime(),
		LastSignalTime: h.engine.GetLastSignalTime(),
		CurrentAIScore: h.engine.GetCurrentAIScore(),
		AIScoreDecay:   h.engine.GetAIScoreDecay(),
		OpenOrders:     h.engine.GetOpenOrders(),
	}, nil
}

// GetPositions 查询当前持仓
func (h *TradingAgentHandler) GetPositions(ctx context.Context, req *PositionRequest) (*PositionList, error) {
	positions := h.positionMgr.GetPositions()
	posList := make([]Position, 0, len(positions))
	for _, p := range positions {
		posList = append(posList, Position(p))
	}
	return &PositionList{
		Positions: posList,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

// EmergencyClose 紧急平仓
func (h *TradingAgentHandler) EmergencyClose(ctx context.Context, req *EmergencyCloseRequest) (*EmergencyCloseResponse, error) {
	symbol := req.Symbol
	if symbol == "" {
		symbol = "all"
	}

	reason := req.Reason
	if reason == "" {
		reason = "manual emergency close"
	}

	// 进入安全模式 + 实际平仓
	if err := h.engine.EnterSafeMode(fmt.Sprintf("%s (by %s)", reason, req.RequestBy)); err != nil {
		return &EmergencyCloseResponse{
			Success: false,
			Message: fmt.Sprintf("failed to enter safe mode: %v", err),
		}, nil
	}

	// 调用引擎执行实际平仓操作
	if err := h.engine.EmergencyClosePositions(); err != nil {
		return &EmergencyCloseResponse{
			Success: false,
			Message: fmt.Sprintf("emergency close order failed: %v", err),
		}, nil
	}

	return &EmergencyCloseResponse{
		Success: true,
		Message: fmt.Sprintf("emergency close executed for %s", symbol),
	}, nil
}

// PauseStrategy 暂停策略
func (h *TradingAgentHandler) PauseStrategy(ctx context.Context, req *PauseStrategyRequest) (*PauseStrategyResponse, error) {
	name := req.StrategyName
	if name == "" {
		name = "all"
	}

	if err := h.engine.PauseStrategy(name); err != nil {
		return &PauseStrategyResponse{
			Success: false,
			Message: fmt.Sprintf("pause failed: %v", err),
		}, nil
	}

	return &PauseStrategyResponse{
		Success: true,
		Message: fmt.Sprintf("strategy %s paused", name),
	}, nil
}

// ResumeStrategy 恢复策略
func (h *TradingAgentHandler) ResumeStrategy(ctx context.Context, req *ResumeStrategyRequest) (*ResumeStrategyResponse, error) {
	name := req.StrategyName
	if name == "" {
		name = "all"
	}

	if err := h.engine.ResumeStrategy(name); err != nil {
		return &ResumeStrategyResponse{
			Success: false,
			Message: fmt.Sprintf("resume failed: %v", err),
		}, nil
	}

	return &ResumeStrategyResponse{
		Success: true,
		Message: fmt.Sprintf("strategy %s resumed", name),
	}, nil
}

// UpdateRiskParams 修改风控参数
func (h *TradingAgentHandler) UpdateRiskParams(ctx context.Context, req *UpdateRiskRequest) (*UpdateRiskResponse, error) {
	if err := h.riskMgr.UpdateRiskParams(req.RiskParam, req.NewValue); err != nil {
		return &UpdateRiskResponse{
			Success: false,
			Message: fmt.Sprintf("update failed: %v", err),
		}, nil
	}

	return &UpdateRiskResponse{
		Success: true,
		Message: fmt.Sprintf("risk param %s updated to %v", req.RiskParam, req.NewValue),
	}, nil
}

// HealthCheck gRPC 健康检查（区别于 HTTP /healthz）
func (h *TradingAgentHandler) HealthCheck(ctx context.Context, req *HealthRequest) (*HealthCheckResponse, error) {
	uptime := int64(time.Since(h.startTime).Seconds())

	// 通过 engine 状态判断整体健康度
	status := "healthy"
	engineStatus := h.engine.GetStatus()
	if engineStatus == "safe_mode" || engineStatus == "paused" {
		status = "degraded"
	}

	deps := []DependencyStatus{
		{Name: "engine", Status: "ok"},
	}

	return &HealthCheckResponse{
		Service: h.serviceName,
		Status:  status,
		Version: h.version,
		Uptime:  uptime,
		Deps:    deps,
	}, nil
}

// ============================================================
// SubscribeStateChanges (TODO)
//
// 需要 proto server stream 生成后才能实现。当前 orchestrator.proto
// 未定义此 RPC 的消息类型和 stream 定义，预留接口：
//
//	// proto 定义示例（待补充到 orchestrator.proto）：
//	service OrchestratorService {
//	  rpc SubscribeStateChanges(SubscribeRequest) returns (stream StateChange);
//	}
//	message SubscribeRequest {
//	  repeated string event_types = 1;  // 订阅的事件类型
//	}
//	message StateChange {
//	  string event_type = 1;            // "position_change" / "mode_change" / "alert"
//	  bytes  payload   = 2;            // JSON 序列化的具体数据
//	  int64  timestamp = 3;
//	}
//
// 实现步骤：
//  1. 在 contracts/orchestrator.proto 中定义 SubscribeRequest / StateChange 消息
//  2. make gen 生成 Go stub
//  3. 在 TradingAgentServer 接口追加 SubscribeStateChanges 方法
//  4. 使用 grpc.ServerStream 推送 Stream
//  5. orchestrator 端 stream.go 接收后转发 WebSocket
// ============================================================
