# orchestrator — 问题记录

> 记录本模块开发过程中发现的问题和修复方式。每次修复后同步更新此文件。

---

## 当前待解决问题

| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-14 | gRPC Invoke 使用 JSON→proto 透传（rawCodec），需等 proto 生成后切到类型安全 stub | 🟡 | open |
| 2 | 2026-06-14 | gRPC 流订阅（SubscribeStateChanges）为骨架，待 trading_agent 实现 RPC 后接入 | 🟡 | open |
| 3 | 2026-06-14 | REST 端点不完整（缺 /services /orders /trades /ai/scores 等），见 PENDING.md | 🟢 | open |
| 4 | 2026-06-14 | WebSocket Hub + Push Broker 未实现，见 PENDING.md | 🟢 | open |
| 5 | 2026-06-14 | CORS 中间件未实现，见 PENDING.md | 🟢 | open |
| 6 | 2026-06-14 | JWT 认证中间件未实现（rest/auth.go 桩），见 PENDING.md | 🟢 | open |

---

## 已修复问题

| # | 发现时间 | 修复时间 | 描述 | 修复方式 | 版本 |
|---|---|---|---|---|---|
| 1 | 2026-06-14 | 2026-06-14 | 🔴 所有适配器和服务注册为桩代码，cmd/main.go 仅硬编码 HTTP 服务器 | 完整实现 config/redis/gRPC/registry/healthz/graceful shutdown | v0.3.0 |
| 2 | 2026-06-14 | 2026-06-14 | 🔴 /healthz 硬编码 `healthy`，不检查任何依赖 | HealthChecker 注入 Redis/TradingPing，返回 degraded + 依赖状态 | v0.3.0 |
| 3 | 2026-06-14 | 2026-06-14 | 🔴 无服务注册（watchdog 无法发现） | pkg/registry.Register 三元组注册 | v0.3.0 |
| 4 | 2026-06-14 | 2026-06-14 | 🔴 无优雅关闭（http.ListenAndServe 阻塞，无 SIGTERM） | http.Server + signal.Notify + 超时控制 | v0.3.0 |
| 5 | 2026-06-14 | 2026-06-14 | 🔴 交易时段守卫为空桩（pass-through） | 实现 domain.IsInTradingSession 检查，非 GET 拦截，紧急平仓例外 | v0.3.0 |
| 6 | 2026-06-14 | 2026-06-14 | 🔴 代码审查：`audit_writer` 引用不存在的 `entry.Detail` 字段 → 编译失败 | 改用 `OldValue`/`NewValue`，格式改为 Line Protocol（避免嵌套 map） | v0.3.1 |
| 7 | 2026-06-14 | 2026-06-14 | 🔴 代码审查：`Ping()` 使用 `WaitForStateChange`，健康连接阻塞到超时后返回 error | 改用 `GetState()` 直接检查 `Ready/Idle` | v0.3.1 |
| 8 | 2026-06-14 | 2026-06-14 | 🔴 代码审查：`TradingPing` 闭包只判 nil 不调 `Ping()`，gRPC 断连后 /healthz 仍报 ok | 改为调用 `tradingClient.Ping(ctx)` | v0.3.1 |
| 9 | 2026-06-14 | 2026-06-14 | 🔴 代码审查：`TradingSessionGuard` 未接入 HTTP 中间件链 | import `rest` + 包裹 mux | v0.3.1 |
| 10 | 2026-06-14 | 2026-06-14 | 🔴 代码审查：gRPC Invoke 发送 `[]byte`，默认 proto codec 不匹配 | 实现 `rawCodec` + `grpc.ForceCodec` 透传 | v0.3.1 |
| 11 | 2026-06-14 | 2026-06-14 | 🟡 代码审查：`Stream.Stop()` 重复调用 panic（close of closed channel） | `sync.Once` 保护 | v0.3.1 |
| 12 | 2026-06-14 | 2026-06-14 | 🟡 代码审查：`/metrics` uptime 硬编码为 1 | 改为 `time.Since(startTime).Seconds()` | v0.3.1 |
| 13 | 2026-06-14 | 2026-06-14 | 🟡 代码审查：HTTP/registry shutdown 共用同一超时 context | 各步骤独立 context | v0.3.1 |
| 14 | 2026-06-14 | 2026-06-14 | 🟡 代码审查：审计 writer 未 import，所有修改操作无审计记录 | import + 注入到 3 个修改型 handler | v0.3.1 |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 占位初版 |
| v1.1 | 2026-06-14 | 首轮开发：14 个已修复（10🔴 + 4🟡），6 个待解决 |
