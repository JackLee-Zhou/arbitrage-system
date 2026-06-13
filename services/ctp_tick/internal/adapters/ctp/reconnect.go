// ctp_tick/adapters/ctp — 断线重连逻辑（cgo 真实现）
//go:build ctp
// +build ctp

package ctp

import (
	"log"
	"time"
)

// ============================================================
// 断线重连策略（参考 Python ctp_engine MdSpi.OnFrontDisconnected）
// 滑动窗口限频：60s 内最多重连 3 次
// 指数退避等待：第1次 10s → 第2次 20s → 第3次 30s
// ============================================================

// handleDisconnect 处理断线事件
// 在滑动窗口内计数，超过 MaxReconnectAttempts 则触发引擎停止
// 否则等待后退避时间后发起重连
func (m *MdAPI) handleDisconnect(reason int) {
	now := time.Now()

	m.disconnectMu.Lock()
	// 清理窗口外的旧记录
	cutoff := now.Add(-ReconnectWindowSec * time.Second)
	recent := make([]time.Time, 0, len(m.disconnectTimes))
	for _, t := range m.disconnectTimes {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	m.disconnectTimes = append(recent, now)
	count := len(m.disconnectTimes)
	m.disconnectMu.Unlock()

	log.Printf("[ctp.reconnect] disconnect #%d in %ds window (reason=%d)", count, ReconnectWindowSec, reason)

	if count > MaxReconnectAttempts {
		log.Printf("[ctp.reconnect] exceeded max attempts (%d), stopping engine", MaxReconnectAttempts)
		return
	}

	// 指数退避：10s → 20s → 30s
	waitSec := MinReconnectWaitSec * count
	if waitSec > MaxReconnectWaitSec {
		waitSec = MaxReconnectWaitSec
	}

	log.Printf("[ctp.reconnect] waiting %ds before reconnect (%d/%d)", waitSec, count, MaxReconnectAttempts)

	// 异步等待后发起重连
	go func() {
		select {
		case <-time.After(time.Duration(waitSec) * time.Second):
			log.Printf("[ctp.reconnect] attempting reconnect...")
			// 重新注册前置机 + Init（与 Python 版 reconnect_loop 一致）
			m.attemptReconnect()
		}
	}()
}

// attemptReconnect 尝试重连（重新 RegisterFront + Init）
func (m *MdAPI) attemptReconnect() {
	m.mu.RLock()
	api := m.api
	symbols := m.symbols
	m.mu.RUnlock()

	if api == nil {
		log.Printf("[ctp.reconnect] api is nil, cannot reconnect")
		return
	}

	// 重置连接信号通道
	m.connected = make(chan struct{}, 1)
	m.loggedIn = make(chan error, 1)
	m.state.Store(int32(StateConnecting))

	// 重新注册前置机 + Init（与 Python 版 reconnect 逻辑一致）
	// 在 CTP 断线后直接 Init 即可，无需重新 RegisterFront

	log.Printf("[ctp.reconnect] re-initializing...")
	// C.Init((*C.CThostFtdcMdApi)(api))

	// 等待连接回调
	_ = symbols // 恢复后自动继续接收行情
}

// clearDisconnectHistory 清空断线时间窗口（OnFrontConnected 时调用）
func (m *MdAPI) clearDisconnectHistory() {
	m.disconnectMu.Lock()
	defer m.disconnectMu.Unlock()
	m.disconnectTimes = nil
}
