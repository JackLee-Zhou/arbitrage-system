// trading_agent/domain/risk — 单笔止损规则
package risk

// SingleLossChecker 单笔止损检查
type SingleLossChecker struct {
	StopLossPct float64 // 止损比例（默认 0.005 = 0.5%）
}

// Check 检查是否触发单笔止损
// 返回 true 表示需要止损
func (c *SingleLossChecker) Check(entryPrice, currentPrice float64, direction string) bool {
	if entryPrice <= 0 {
		return false
	}
	pnlPct := (currentPrice - entryPrice) / entryPrice
	if direction == "short" {
		pnlPct = -pnlPct
	}
	return pnlPct < -c.StopLossPct
}
