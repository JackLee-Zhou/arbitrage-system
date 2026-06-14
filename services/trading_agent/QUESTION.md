# trading_agent — 问题记录

> 本模块尚未开始正式开发。开发前必须先读本文档，了解上游 orchestrator 对本模块的依赖和接口契约。

## 当前待解决问题

| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-14 | 🔴 **orchestrator 依赖 proto 生成**：orchestrator 的 gRPC 客户端当前使用 rawCodec（JSON 透传）。trading_agent 开发时需 `make gen` 生成 orchestrator.proto 的 Go stub，然后 orchestrator 切到 proto stub client | 🔴 | open |
| 2 | 2026-06-14 | 🔴 **须实现 SubscribeStateChanges RPC**（server stream）：orchestrator 通过此接口接收状态变化推送（持仓/告警/引擎状态），不再轮询。orchestrator 端的 stream.go 已预留接口，实现后 orchestrator 改为接收推送 → 转发 WebSocket | 🔴 | open |
| 3 | 2026-06-14 | 🔴 **须注册到 Redis**：调用 pkg/registry.Register，写入三元组（host:port + name + capabilities），供 watchdog 自动发现。orchestrator、ctp_tick、db_writer 均已实现，参考它们 | 🔴 | open |
| 4 | 2026-06-14 | 🟡 **/healthz 须返回依赖检查**：Redis + CTP 连通性状态，watchdog 和 orchestrator 都会探测此端点。参考 orchestrator/api/healthz.go 的 HealthChecker 模式 | 🟡 | open |
| 5 | 2026-06-14 | 🟡 **/metrics 须输出 Prometheus 指标**（非硬编码占位），所有 Go 服务的统一要求 | 🟡 | open |

---

## 已修复问题

| # | 发现时间 | 修复时间 | 描述 | 修复方式 | 版本 |
|---|---|---|---|---|---|

---

## orchestrator 接口契约（开发 trading_agent 时必须满足）

### gRPC RPC（proto 定义见 contracts/orchestrator.proto）

| RPC | 用途 | orchestrator 端状态 |
|-----|------|-------------------|
| `GetStatus` | 查询引擎状态 | ✅ 已调用（rawCodec） |
| `GetPositions` | 查询当前持仓 | ✅ 已调用（rawCodec） |
| `EmergencyClose` | 紧急平仓 | ✅ 已调用（rawCodec） |
| `PauseStrategy` | 暂停策略 | ✅ 已调用（rawCodec） |
| `ResumeStrategy` | 恢复策略 | ✅ 已调用（rawCodec） |
| `UpdateRiskParams` | 更新风控参数 | ✅ 已调用（rawCodec） |
| `HealthCheck` | 健康检查（gRPC） | ⏳ 待接入 |
| `SubscribeStateChanges` | 状态变化流推送 | ⏳ 骨架等待 |

### Redis 注册格式

```json
{
  "name": "trading_agent",
  "health_url": "trading_agent:8081",
  "metrics_url": "trading_agent:8081",
  "capabilities": ["trading", "strategy", "risk_control", "indicator"]
}
```

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 占位初版 |
| v1.1 | 2026-06-14 | 记录 orchestrator 对本模块的 5 个待解决问题 + gRPC/Redis 接口契约 |
