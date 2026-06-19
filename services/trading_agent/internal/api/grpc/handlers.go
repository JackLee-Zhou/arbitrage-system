// trading_agent/api/grpc — gRPC 请求处理器
//
// 实现 contracts/orchestrator.proto 定义的 OrchestratorService 7 个 RPC。
// 使用 protoc 生成的 arbiragev1 包中的消息类型和服务接口。
package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/arbitrage-system/contracts/generated/go/arbitragev1"
)

// ============================================================
// 接口定义（依赖注入，避免循环依赖）
// ============================================================

// EngineAccessor 引擎状态访问接口
type EngineAccessor interface {
	GetStatus() string
	GetLastTickTime() int64
	GetLastSignalTime() int64
	GetCurrentAIScore() float64
	GetAIScoreDecay() float64
	GetOpenOrders() int32
	PauseStrategy(name string) error
	ResumeStrategy(name string) error
	EnterSafeMode(reason string) error
	EmergencyClosePositions() error
}

// PositionAccessor 持仓访问接口
type PositionAccessor interface {
	GetPositions() []PositionInfo
}

// PositionInfo 持仓信息（API 层视图）
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
	UpdateRiskParams(param string, value float64) error
}

// ============================================================
// TradingAgentHandler — 实现 arbiragev1.OrchestratorServiceServer
// ============================================================

// TradingAgentHandler 实现生成的 OrchestratorServiceServer 接口
type TradingAgentHandler struct {
	arbitragev1.UnimplementedOrchestratorServiceServer
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
// RPC 方法 — 使用 proto 生成的消息类型
// ============================================================

func (h *TradingAgentHandler) GetStatus(ctx context.Context, req *arbitragev1.StatusRequest) (*arbitragev1.StatusResponse, error) {
	return &arbitragev1.StatusResponse{
		EngineStatus:   h.engine.GetStatus(),
		LastTickTime:   h.engine.GetLastTickTime(),
		LastSignalTime: h.engine.GetLastSignalTime(),
		CurrentAiScore: h.engine.GetCurrentAIScore(),
		AiScoreDecay:   h.engine.GetAIScoreDecay(),
		OpenOrders:     h.engine.GetOpenOrders(),
	}, nil
}

func (h *TradingAgentHandler) GetPositions(ctx context.Context, req *arbitragev1.PositionRequest) (*arbitragev1.PositionList, error) {
	positions := h.positionMgr.GetPositions()
	posList := make([]*arbitragev1.Position, 0, len(positions))
	for _, p := range positions {
		posList = append(posList, &arbitragev1.Position{
			Symbol:    p.Symbol,
			Direction: p.Direction,
			Volume:    p.Volume,
			AvgPrice:  p.AvgPrice,
			LastPrice: p.LastPrice,
			FloatPnl:  p.FloatPnl,
		})
	}
	return &arbitragev1.PositionList{
		Positions: posList,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

func (h *TradingAgentHandler) EmergencyClose(ctx context.Context, req *arbitragev1.EmergencyCloseRequest) (*arbitragev1.EmergencyCloseResponse, error) {
	symbol := req.GetSymbol()
	if symbol == "" {
		symbol = "all"
	}
	reason := req.GetReason()
	if reason == "" {
		reason = "manual emergency close"
	}

	if err := h.engine.EnterSafeMode(fmt.Sprintf("%s (by %s)", reason, req.GetRequestBy())); err != nil {
		return &arbitragev1.EmergencyCloseResponse{
			Success: false,
			Message: fmt.Sprintf("failed to enter safe mode: %v", err),
		}, nil
	}

	if err := h.engine.EmergencyClosePositions(); err != nil {
		return &arbitragev1.EmergencyCloseResponse{
			Success: false,
			Message: fmt.Sprintf("emergency close order failed: %v", err),
		}, nil
	}

	return &arbitragev1.EmergencyCloseResponse{
		Success: true,
		Message: fmt.Sprintf("emergency close executed for %s", symbol),
	}, nil
}

func (h *TradingAgentHandler) PauseStrategy(ctx context.Context, req *arbitragev1.PauseStrategyRequest) (*arbitragev1.PauseStrategyResponse, error) {
	name := req.GetStrategyName()
	if name == "" {
		name = "all"
	}
	if err := h.engine.PauseStrategy(name); err != nil {
		return &arbitragev1.PauseStrategyResponse{
			Success: false,
			Message: fmt.Sprintf("pause failed: %v", err),
		}, nil
	}
	return &arbitragev1.PauseStrategyResponse{
		Success: true,
		Message: fmt.Sprintf("strategy %s paused", name),
	}, nil
}

func (h *TradingAgentHandler) ResumeStrategy(ctx context.Context, req *arbitragev1.ResumeStrategyRequest) (*arbitragev1.ResumeStrategyResponse, error) {
	name := req.GetStrategyName()
	if name == "" {
		name = "all"
	}
	if err := h.engine.ResumeStrategy(name); err != nil {
		return &arbitragev1.ResumeStrategyResponse{
			Success: false,
			Message: fmt.Sprintf("resume failed: %v", err),
		}, nil
	}
	return &arbitragev1.ResumeStrategyResponse{
		Success: true,
		Message: fmt.Sprintf("strategy %s resumed", name),
	}, nil
}

func (h *TradingAgentHandler) UpdateRiskParams(ctx context.Context, req *arbitragev1.UpdateRiskRequest) (*arbitragev1.UpdateRiskResponse, error) {
	if err := h.riskMgr.UpdateRiskParams(req.GetRiskParam(), req.GetNewValue()); err != nil {
		return &arbitragev1.UpdateRiskResponse{
			Success: false,
			Message: fmt.Sprintf("update failed: %v", err),
		}, nil
	}
	return &arbitragev1.UpdateRiskResponse{
		Success: true,
		Message: fmt.Sprintf("risk param %s updated to %v", req.GetRiskParam(), req.GetNewValue()),
	}, nil
}

func (h *TradingAgentHandler) HealthCheck(ctx context.Context, req *arbitragev1.HealthRequest) (*arbitragev1.HealthCheckResponse, error) {
	uptime := int64(time.Since(h.startTime).Seconds())

	status := "healthy"
	engineStatus := h.engine.GetStatus()
	if engineStatus == "safe_mode" || engineStatus == "paused" {
		status = "degraded"
	}

	return &arbitragev1.HealthCheckResponse{
		Service: h.serviceName,
		Status:  status,
		Version: h.version,
		Uptime:  uptime,
	}, nil
}
