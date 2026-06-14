# orchestrator — 架构与业务流

> 编排层（面向人类的代理）：REST + WebSocket API 网关，上游是 dashboard_web，下游是 trading_agent / Redis / InfluxDB。

## 服务定位

```
dashboard_web (React)
      │ HTTP REST + WebSocket
      ▼
┌─────────────────────────┐
│ orchestrator (:8080)     │ ← 你在这里
│ • REST API 网关           │
│ • gRPC 客户端 → trading_agent
│ • Redis 只读聚合          │
│ • 操作审计                │
│ • 交易时段守卫            │
│ • 服务健康聚合            │
└─────────────────────────┘
      │ gRPC :9090          │ Redis 只读
      ▼                     ▼
trading_agent           Redis 7.2
(决策核心)               (AI评分/持仓/引擎状态/服务注册)
```

## 启动流程

```
main()
  ├─ 1. 加载配置（环境变量）
  ├─ 2. 连接 Redis（只读）
  ├─ 3. 连接 trading_agent gRPC（失败降级，不阻塞启动）
  ├─ 4. 创建审计日志 writer
  ├─ 5. 启动服务健康巡检器 HealthPoller（后台 goroutine，每 15s）
  ├─ 6. 注册到 Redis（三元组：host:port + name + capabilities）
  ├─ 7. 装配 HTTP Handler
  │     ├─ /healthz      → HealthChecker（Redis + gRPC 依赖检查）
  │     ├─ /metrics      → Prometheus 文本
  │     ├─ /api/v1/overview    → 聚合 AI 评分 + 引擎状态 + 服务健康
  │     ├─ /api/v1/positions   → gRPC → Redis 兜底
  │     ├─ /api/v1/emergency_close → gRPC + 审计日志
  │     ├─ /api/v1/strategies  → gRPC + 审计日志
  │     ├─ /api/v1/risk        → gRPC + 审计日志
  │     └─ /ws                 → WebSocket（占位）
  ├─ 8. 中间件链：日志 → 交易时段守卫 → 路由
  ├─ 9. 启动 HTTP Server（:8080）
  ├─ 10. 等待 SIGTERM
  └─ 11. 优雅关闭（HTTP → gRPC → Redis）
```

## 业务流

### 查询流（GET /api/v1/overview）

```
dashboard_web
  │ GET /api/v1/overview
  ▼
orchestrator
  ├─ 读内存缓存 → 服务健康列表（HealthPoller.Snapshot()）
  ├─ 读 Redis → AI 评分（ai_score:MA2509:live）
  └─ 调 gRPC → trading_agent.GetStatus() → 引擎状态
  │ 聚合返回 JSON
  ▼
dashboard 展示：引擎灯 + 仓位 + AI 评分 + 各服务健康灯
```

### 修改流（POST /api/v1/emergency_close）

```
dashboard_web
  │ POST /api/v1/emergency_close {"symbol":"MA2509","reason":"手动平仓"}
  ▼
withMiddleware（记录请求日志）
  ▼
TradingSessionGuard（紧急平仓例外，直接放行）
  ▼
handler
  ├─ tradingClient.EmergencyClose(ctx, reason, symbol)
  │     │ gRPC → trading_agent:9090
  │     ▼
  │   trading_agent 执行平仓 → 返回 order_id
  └─ auditWriter.Write(ctx, AuditEntry{...})
        │ XADD write:audit
        ▼
      Redis Stream → db_writer → InfluxDB audit_log
```

### 服务健康聚合流

```
HealthPoller（每 15s 后台 goroutine）
  │
  ├─ 1. discover(): Redis KEYS services:* → 解析 health_url
  │      services:ctp_tick  → localhost:8081
  │      services:db_writer → localhost:8082
  │      services:orchestrator → localhost:8080
  │
  ├─ 2. 并发 probe(): HTTP GET http://<health_url>/healthz
  │      解析 status (healthy/degraded/unhealthy)
  │
  └─ 3. 缓存到内存 services map
        │
        ▼
      /api/v1/overview 读取 Snapshot()
```

## 核心模块

| 模块 | 文件 | 职责 |
|------|------|------|
| 配置 | `adapters/config/config.go` | 环境变量加载 |
| Redis 读取器 | `adapters/redis/reader.go` | 只读查询（AI 评分/持仓/引擎状态/服务列表） |
| gRPC 客户端 | `adapters/trading_client/client.go` | 6 个 RPC + rawCodec 透传 + Ping |
| gRPC 流 | `adapters/trading_client/stream.go` | SubscribeStateChanges 骨架 |
| 审计日志 | `adapters/audit_writer/writer.go` | Line Protocol 写入 Redis Stream |
| 健康巡检 | `adapters/health_poller.go` | Redis 发现 + HTTP 探测 + 结果缓存 |
| 健康检查 | `api/healthz.go` | /healthz，Redis + gRPC 依赖检查 |
| 交易时段守卫 | `api/rest/middleware.go` | 拦截修改操作，紧急平仓例外 |
| 入口 | `cmd/main.go` | DI 装配 + 注册 + 优雅关闭 |

## 中间件链

```
请求进入
  │
  ├─ withMiddleware          ← 记录请求日志
  ├─ TradingSessionGuard     ← GET/WS/emergency_close: 放行
  │                           ← POST/PUT/DELETE: 交易时段内拒绝(403)
  ▼                           ← 非交易时段: 放行
mux 路由 → handler
```

## 外部依赖

| 依赖 | 地址 | 用途 | 必需 |
|------|------|------|------|
| Redis | `REDIS_ADDR` | 读写服务注册/业务数据/审计流 | ✅ |
| trading_agent | `TRADING_AGENT_GRPC` | gRPC 调用决策引擎 | ⚠️ 失败降级 |
| InfluxDB | 经 db_writer | 审计日志持久化 | 间接 |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ORCHESTRATOR_SERVICE_NAME` | `orchestrator` | 注册名 |
| `ORCHESTRATOR_SERVICE_HOST` | 空→容器名 | 本机可达地址 |
| `HTTP_PORT` | `8080` | HTTP 端口 |
| `REDIS_ADDR` | `redis:6379` | Redis 地址 |
| `TRADING_AGENT_GRPC` | `trading_agent:9090` | gRPC 地址 |
| `SCRAPE_INTERVAL` | `5s` | （预留，健康巡检间隔在 HealthPoller 硬编码 15s） |
