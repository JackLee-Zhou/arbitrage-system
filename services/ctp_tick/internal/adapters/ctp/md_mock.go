// ctp_tick/adapters/ctp — Mock 行情订阅（macOS 本地开发用）
//go:build !ctp

package ctp

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// MockMdAPI — 本地开发模拟行情（实现 MarketDataSubscriber 接口）
// 模拟 CTP 回调节奏，生成随机 Tick 数据用于开发调试
// ============================================================

type MockMdAPI struct {
	frontAddr  string
	brokerID   string
	userID     string
	symbols    []string
	handler    TickHandler
	state      atomic.Int32
	tradingDay string

	stopCh chan struct{}
	tickCh chan struct{}
	mu     sync.RWMutex
}

// NewMarketDataSubscriber 创建 Mock 行情订阅实例
func NewMarketDataSubscriber(flowPath, frontAddr, brokerID, userID, password string) MarketDataSubscriber {
	return newMockMdAPI(flowPath, frontAddr, brokerID, userID, password)
}

func newMockMdAPI(flowPath, frontAddr, brokerID, userID, password string) *MockMdAPI {
	m := &MockMdAPI{
		frontAddr:  frontAddr,
		brokerID:   brokerID,
		userID:     userID,
		stopCh:     make(chan struct{}),
		tickCh:     make(chan struct{}, 1),
		tradingDay: time.Now().Format("20060102"),
	}
	m.state.Store(int32(StateDisconnected))
	return m
}

// Connect Mock 连接：模拟完整的连接 → 登录 → 订阅流程
func (m *MockMdAPI) Connect(symbols []string) error {
	m.symbols = symbols

	// 模拟 OnFrontConnected
	m.state.Store(int32(StateConnecting))
	time.Sleep(100 * time.Millisecond)
	log.Printf("[ctp.mock] OnFrontConnected (mock)")
	m.state.Store(int32(StateConnected))

	// 模拟登录
	time.Sleep(50 * time.Millisecond)
	log.Printf("[ctp.mock] ReqUserLogin: broker=%s user=%s (mock)", m.brokerID, m.userID)
	m.state.Store(int32(StateLoggedIn))

	// 模拟订阅
	log.Printf("[ctp.mock] SubscribeMarketData: %v (mock)", symbols)
	m.state.Store(int32(StateSubscribed))

	if len(symbols) == 0 {
		return fmt.Errorf("ctp.mock: no symbols to subscribe")
	}

	// 启动 mock Tick 推送协程
	go m.tickLoop()

	return nil
}

// Close 停止 mock 推送
func (m *MockMdAPI) Close() {
	close(m.stopCh)
	m.state.Store(int32(StateDisconnected))
	log.Printf("[ctp.mock] api released")
}

// OnTick 注册行情回调
func (m *MockMdAPI) OnTick(handler TickHandler) {
	m.mu.Lock()
	m.handler = handler
	m.mu.Unlock()
}

// IsConnected 返回是否已连接
func (m *MockMdAPI) IsConnected() bool {
	return m.state.Load() >= int32(StateConnected)
}

// TradingDay 返回当前交易日
func (m *MockMdAPI) TradingDay() string {
	return m.tradingDay
}

// GetState 返回连接状态
func (m *MockMdAPI) GetState() ConnectionState {
	return ConnectionState(m.state.Load())
}

// ============================================================
// Mock Tick 生成
// ============================================================

// tickLoop 模拟行情推送循环（约 2 笔/秒，与真实甲醇主力频率一致）
func (m *MockMdAPI) tickLoop() {
	basePrice := 2350.0                              // 甲醇基准价
	ticker := time.NewTicker(500 * time.Millisecond) // 每秒 2 笔
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.generateTick(basePrice)
		}
	}
}

// generateTick 生成一条模拟行情数据
func (m *MockMdAPI) generateTick(basePrice float64) {
	m.mu.RLock()
	handler := m.handler
	symbols := m.symbols
	m.mu.RUnlock()

	if handler == nil || len(symbols) == 0 {
		return
	}

	// 随机波动 ±0.5%
	delta := basePrice * 0.005 * (rand.Float64()*2 - 1)

	data := &DepthMarketData{
		TradingDay:         time.Now().Format("20060102"),
		InstrumentID:       symbols[0],
		UpdateTime:         time.Now().Format("15:04:05"),
		LastPrice:          basePrice + delta,
		Volume:             100000 + rand.Int63n(50000),
		OpenPrice:          basePrice - 5,
		HighestPrice:       basePrice + 10,
		LowestPrice:        basePrice - 10,
		PreSettlementPrice: basePrice,
		OpenInterest:       500000,
		BidPrice1:          basePrice + delta - 1,
		BidVolume1:         10 + rand.Int63n(50),
		AskPrice1:          basePrice + delta + 1,
		AskVolume1:         10 + rand.Int63n(50),
	}

	handler(data)
}
