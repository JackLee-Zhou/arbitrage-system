// ctp_tick/adapters/ctp — CTP 行情适配器（cgo 封装）
//go:build ctp
// +build ctp

package ctp

// 本文件在 macOS 本地开发时不编译（需要 Linux + CTP .so）
// 使用 go build -tags ctp 编译
// TODO: 实现 openctp/ctp-go 的行情订阅回调
