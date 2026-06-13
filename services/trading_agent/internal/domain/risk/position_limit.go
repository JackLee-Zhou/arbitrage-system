// trading_agent/domain/risk — 仓位上限限制
package risk

// PositionLimit 仓位上限
type PositionLimit struct {
	MaxPosition int // 最大持仓手数（默认 2）
	Current     int // 当前持仓
}

// Check 检查是否超出仓位上限
func (l *PositionLimit) Check(proposedVolume int) bool {
	return l.Current+proposedVolume <= l.MaxPosition
}
