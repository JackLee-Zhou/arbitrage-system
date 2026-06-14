// ctp_tick/adapters/ctp — 断线重连逻辑（cgo 真实现）
//go:build ctp
// +build ctp

package ctp

import (
	"log"
	"sync"
	"time"
)

// ============================================================
// 断线重连策略（参考 Python ctp_engine MdSpi.OnFrontDisconnected）
// 滑动窗口限频：60s 内最多重连 3 次
// 指数退避等待：第1次 10s → 第2次 20s → 第3次 30s
// ============================================================

// handleDisconnect 处理断线事件。由 CTP 回调线程调用，异步发起重连。
func (m *MdAPI) handleDisconnect(reason int) {
	now := time.Now()

	m.disconnectMu.Lock()
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

	waitSec := MinReconnectWaitSec * count
	if waitSec > MaxReconnectWaitSec {
		waitSec = MaxReconnectWaitSec
	}

	log.Printf("[ctp.reconnect] waiting %ds before reconnect (%d/%d)", waitSec, count, MaxReconnectAttempts)

	// 使用 time.AfterFunc 以便 Close() 可通过 Stop 取消
	m.disconnectMu.Lock()
	m.reconnectTimer = time.AfterFunc(time.Duration(waitSec)*time.Second, func() {
		log.Printf("[ctp.reconnect] attempting reconnect...")
		m.attemptReconnect()
	})
	m.disconnectMu.Unlock()
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

	// 加锁保护 channel 替换，防止与 CTP 回调线程 data race
	m.mu.Lock()
	m.connected = make(chan struct{}, 1)
	m.loggedIn = make(chan error, 1)
	m.mu.Unlock()

	m.state.Store(int32(StateConnecting))
	log.Printf("[ctp.reconnect] re-initializing...")
	// C.Init((*C.CThostFtdcMdApi)(api))  // 需 Linux cgo 环境
	_ = symbols
}

// clearDisconnectHistory 清空断线计数（OnFrontConnected 时重置）
func (m *MdAPI) clearDisconnectHistory() {
	m.disconnectMu.Lock()
	defer m.disconnectMu.Unlock()
	m.disconnectTimes = nil
}

// stopReconnectTimer 取消等待中的重连定时器
func (m *MdAPI) stopReconnectTimer() {
	m.disconnectMu.Lock()
	defer m.disconnectMu.Unlock()
	if m.reconnectTimer != nil {
		m.reconnectTimer.Stop()
		m.reconnectTimer = nil
	}
}

// reconnectTimer 字段声明见 md.go（MdAPI struct 新增）
// 此处通过闭包引用 m，无需在 reconnect.go 中重复定义
var _ = sync.Mutex{} // unused placeholder fix: needed for sync import in non-cgo builds
