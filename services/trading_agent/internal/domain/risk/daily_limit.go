// trading_agent/domain/risk — 日最大亏损限制
package risk

// DailyLossLimit 日最大亏损
type DailyLossLimit struct {
	MaxLossPct float64 // 日最大亏损比例（默认 0.02 = 2%）
	Capital    float64 // 初始资金
	DailyPnL   float64 // 当日累计盈亏
}

// Check 检查是否触发日最大亏损
func (l *DailyLossLimit) Check() bool {
	if l.Capital <= 0 {
		return false
	}
	maxLoss := l.Capital * l.MaxLossPct
	return l.DailyPnL < -maxLoss
}

// AddPnL 累加当日盈亏
func (l *DailyLossLimit) AddPnL(pnl float64) {
	l.DailyPnL += pnl
}

// Reset 重置当日累计（每日开盘时调用）
func (l *DailyLossLimit) Reset() {
	l.DailyPnL = 0
}
