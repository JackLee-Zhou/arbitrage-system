// ctp_tick/domain — 全市场最新价快照（纯业务逻辑，零外部依赖）
package domain

import (
	"sync"
	"time"
)

// ============================================================
// 常量（禁止魔法数字 — ENGINEERING_GUIDE.md 规则 6）
// ============================================================

const (
	// MaxChangePct 最大涨跌幅百分比，用于校验防止异常值
	MaxChangePct = 100.0
)

// ============================================================
// PriceSnapshot — 单个合约的最新价快照
// ============================================================

// PriceSnapshot 维护全市场每个合约的最新价快照。
// 所有字段是覆盖式的，每次 Tick 更新替换旧值。
type PriceSnapshot struct {
	Symbol       string    `json:"symbol"`
	LastPrice    float64   `json:"last_price"`
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	PreClose     float64   `json:"pre_close"`
	ChangePct    float64   `json:"change_pct"`
	Volume       int64     `json:"volume"`
	OpenInterest float64   `json:"open_interest"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ============================================================
// LastPriceStore — 全市场最新价快照存储（线程安全）
// ============================================================

// LastPriceStore 管理 map[symbol]*PriceSnapshot，每个 Tick 触发更新。
// domain 层纯逻辑，依赖 sync.RWMutex 保证并发安全。
type LastPriceStore struct {
	mu sync.RWMutex
	m  map[string]*PriceSnapshot
}

// NewLastPriceStore 创建全市场快照存储实例。
func NewLastPriceStore() *LastPriceStore {
	return &LastPriceStore{
		m: make(map[string]*PriceSnapshot),
	}
}

// Update 从 StandardTick 更新快照（覆盖式）。
// changePct = (last_price - pre_close) / pre_close * 100。
// pre_close 为 0 时 changePct 为 0（避免除零）。
func (s *LastPriceStore) Update(tick *StandardTick) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var changePct float64
	if tick.PreClose > 0 {
		changePct = (tick.LastPrice - tick.PreClose) / tick.PreClose * 100.0
		if changePct > MaxChangePct || changePct < -MaxChangePct {
			changePct = 0
		}
	}

	// 确定 High/Low：保留历史极端值（不修改输入的 tick）
	high := tick.HighPrice
	low := tick.LowPrice
	existing, exists := s.m[tick.Symbol]
	if exists {
		if high < existing.High {
			high = existing.High
		}
		if low > existing.Low || tick.LowPrice == 0 {
			low = existing.Low
		}
	}

	s.m[tick.Symbol] = &PriceSnapshot{
		Symbol:       tick.Symbol,
		LastPrice:    tick.LastPrice,
		Open:         tick.OpenPrice,
		High:         high,
		Low:          low,
		PreClose:     tick.PreClose,
		ChangePct:    changePct,
		Volume:       tick.Volume,
		OpenInterest: tick.OpenInterest,
		UpdatedAt:    time.Now(),
	}
}

// Snapshot 返回当前所有合约的快照副本（用于 HTTP 端点全量返回）。
func (s *LastPriceStore) Snapshot() map[string]*PriceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*PriceSnapshot, len(s.m))
	for k, v := range s.m {
		cp := *v // 值拷贝防止外部篡改
		result[k] = &cp
	}
	return result
}

// Get 返回单个合约的快照。ok=false 表示尚无数据。
func (s *LastPriceStore) Get(symbol string) (*PriceSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.m[symbol]
	if !ok {
		return nil, false
	}
	cp := *v
	return &cp, true
}

// Len 返回已缓存合约数量。
func (s *LastPriceStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}
