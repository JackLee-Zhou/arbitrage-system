// orchestrator/domain — UI 指令路由（纯逻辑）
// 把 dashboard 的 REST 操作转换成对 trading_agent 的 gRPC 指令
package domain

// Command 用户指令
type Command struct {
	Type      string // "emergency_close" / "pause_strategy" / "resume_strategy" / "update_risk"
	Symbol    string
	Strategy  string
	RiskParam string
	NewValue  float64
	RequestBy string
}

// CommandResult 指令执行结果
type CommandResult struct {
	Success bool
	Message string
	OrderIDs []string // 紧急平仓返回的 order ID 列表
}

// RouteCommand 路由指令到正确的后端操作
// 纯逻辑：判断指令类型 → 返回对应 gRPC 方法名 + 参数
func RouteCommand(cmd Command) (grpcMethod string, grpcReq interface{}) {
	switch cmd.Type {
	case "emergency_close":
		return "EmergencyClose", map[string]string{
			"symbol":     cmd.Symbol,
			"reason":     "user_requested",
			"request_by": cmd.RequestBy,
		}
	case "pause_strategy":
		return "PauseStrategy", map[string]string{
			"strategy_name": cmd.Strategy,
			"request_by":    cmd.RequestBy,
		}
	case "resume_strategy":
		return "ResumeStrategy", map[string]string{
			"strategy_name": cmd.Strategy,
			"request_by":    cmd.RequestBy,
		}
	case "update_risk":
		return "UpdateRiskParams", map[string]interface{}{
			"risk_param": cmd.RiskParam,
			"new_value":  cmd.NewValue,
			"request_by": cmd.RequestBy,
		}
	default:
		return "", nil
	}
}
