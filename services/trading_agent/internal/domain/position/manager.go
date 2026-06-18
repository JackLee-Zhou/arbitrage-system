// trading_agent/domain/position — 持仓管理
// 注意：本包零外部依赖
package position

import "sync"

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

// Manager 持仓管理器（线程安全）
type Manager struct {
	mu        sync.RWMutex
	Positions map[string]*Position
}

// Get 获取某个合约的持仓。symbol 不存在时创建并存入 map。
func (m *Manager) Get(symbol string) *Position {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Positions == nil {
		m.Positions = make(map[string]*Position)
	}
	p, ok := m.Positions[symbol]
	if !ok {
		p = &Position{Symbol: symbol}
		m.Positions[symbol] = p
	}
	return p
}

// LoadPosition 从外部数据源加载持仓（如启动对账恢复）。
// 仅在管理器为空时加载，避免覆盖已有状态。
func (m *Manager) LoadPosition(symbol string, dir string, volume int, avgPrice float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Positions == nil {
		m.Positions = make(map[string]*Position)
	}
	// 只有当前无持仓时才加载（不覆盖已有数据）
	if _, ok := m.Positions[symbol]; ok {
		return
	}
	m.Positions[symbol] = &Position{
		Symbol:    symbol,
		Direction: dir,
		Volume:    volume,
		AvgPrice:  avgPrice,
	}
}

// Snapshot 返回所有持仓的快照副本（线程安全，供 gRPC 等并发读取）
func (m *Manager) Snapshot() map[string]Position {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]Position, len(m.Positions))
	for k, v := range m.Positions {
		out[k] = *v // 值拷贝
	}
	return out
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
