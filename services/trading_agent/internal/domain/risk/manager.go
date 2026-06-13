// trading_agent/domain/risk — 风控规则
// 注意：本包零外部依赖，只做纯规则判定
package risk

// RiskCheckResult 风控检查结果
type RiskCheckResult struct {
	Passed  bool
	Reason  string
	Blocked bool // true = 阻止本次操作
}

// Manager 风控管理器
type Manager struct {
	MaxPosition      int     // 最大持仓手数
	MaxLossPerDay    float64 // 单日最大亏损
	StopLossPct      float64 // 单笔止损比例
	DailyPnL         float64 // 当日累计盈亏
	CurrentPosition  int     // 当前持仓
}

// CheckNewOrder 检查是否允许新增报单
func (m *Manager) CheckNewOrder(proposedVolume int, currentPrice, avgPrice float64) *RiskCheckResult {
	// 检查持仓上限
	if m.CurrentPosition+proposedVolume > m.MaxPosition {
		return &RiskCheckResult{Passed: false, Blocked: true, Reason: "max_position_exceeded"}
	}

	// 检查当日亏损上限
	if m.DailyPnL < -m.MaxLossPerDay {
		return &RiskCheckResult{Passed: false, Blocked: true, Reason: "daily_loss_limit"}
	}

	// 检查止损
	if avgPrice > 0 {
		pnlPct := (currentPrice - avgPrice) / avgPrice
		if pnlPct < -m.StopLossPct {
			return &RiskCheckResult{Passed: false, Blocked: true, Reason: "stop_loss_triggered"}
		}
	}

	return &RiskCheckResult{Passed: true}
}

// EmergencyCheck 紧急风控检查
func (m *Manager) EmergencyCheck(priceChangePct1m, volumeMultiplier float64) bool {
	// 1 分钟涨跌幅 > ±1% 或 量比 > 3.0 触发
	return priceChangePct1m > 0.01 || priceChangePct1m < -0.01 || volumeMultiplier > 3.0
}
