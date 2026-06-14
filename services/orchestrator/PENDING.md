# orchestrator — 待实现项

> 以下功能暂不实现，原因和后续计划在此记录。

---

## 暂不实现

| # | 功能 | 原因 | 后续计划 |
|---|------|------|----------|
| 1 | **更多 REST 端点**（/services, /orders, /trades, /ai/scores/*, /strategies CRUD, /risk/params, /audit, /sentiment） | dashboard 尚未对接，按需增量添加 | dashboard 开发时逐端点实现 |
| 2 | **CORS 中间件** | 开发阶段 dashboard 同源部署（nginx 反向代理），暂时不需要跨域 | 前后端分离部署时加 |
| 3 | **WebSocket Hub**（连接管理器） | dashboard 实时推送尚未开发，WebSocket 端点目前占位 | dashboard 实时面板开发时实现 |
| 4 | **WebSocket Push Broker**（消息分发器） | 同上，需要 Hub 作为基础 | 和 Hub 一起实现 |
| 5 | **JWT 认证中间件**（rest/auth.go） | 当前仅内网部署，不需要认证 | 开放公网访问前实现 |

---

## 已完成

| # | 功能 | 文件 |
|---|------|------|
| 1 | 配置加载 | `adapters/config/config.go` |
| 2 | Redis 只读聚合 | `adapters/redis/reader.go` |
| 3 | gRPC 客户端（6 个 RPC） | `adapters/trading_client/client.go` |
| 4 | gRPC 流订阅骨架 | `adapters/trading_client/stream.go` |
| 5 | 审计日志 writer | `adapters/audit_writer/writer.go` |
| 6 | 服务注册（pkg/registry） | `cmd/main.go` |
| 7 | /healthz（依赖检查） | `api/healthz.go` |
| 8 | 交易时段守卫 | `api/rest/middleware.go` |
| 9 | REST API 5 个端点 | `cmd/main.go` |
| 10 | 优雅关闭 | `cmd/main.go` |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0 | 2026-06-14 | 初版，记录 5 项待实现 + 10 项已完成 |
