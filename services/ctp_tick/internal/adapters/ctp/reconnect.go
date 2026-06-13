// ctp_tick/adapters/ctp — 断线重连逻辑
//go:build ctp
// +build ctp

package ctp

// TODO: 实现 CTP 断线检测 + 指数退避重连
// OnFrontDisconnected 回调触发
// 1s → 2s → 4s → 8s ... 最大 60s
// 每日 15:15（日盘收盘）和 23:00（夜盘收盘）后自动进入等待重连
