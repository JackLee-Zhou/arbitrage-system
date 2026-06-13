// trading_agent/domain/position — 持仓管理
// 注意：本包零外部依赖
package position

// Position 持仓信息
type Position struct {
	Symbol      string
	Direction   string // "long" / "short"
	Volume      int
	AvgPrice    float64
	LastPrice   float64
	FloatPnL    float64
	TodayVolume int
	YdVolume    int
}

// Manager 持仓管理器
type Manager struct {
	Positions map[string]*Position
}

// Get 获取某个合约的持仓
func (m *Manager) Get(symbol string) *Position {
	if m.Positions == nil {
		m.Positions = make(map[string]*Position)
	}
	p, ok := m.Positions[symbol]
	if !ok {
		return &Position{Symbol: symbol}
	}
	return p
}

// UpdateFloatPnL 更新浮动盈亏
func (p *Position) UpdateFloatPnL(lastPrice float64) {
	p.LastPrice = lastPrice
	if p.AvgPrice > 0 {
		if p.Direction == "long" {
			p.FloatPnL = float64(p.Volume) * (lastPrice - p.AvgPrice)
		} else {
			p.FloatPnL = float64(p.Volume) * (p.AvgPrice - lastPrice)
		}
	}
}
