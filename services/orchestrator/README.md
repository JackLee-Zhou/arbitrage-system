# services/orchestrator — 编排层（面向人类的代理）

> 本服务属于 arbitrage-system 微服务集合的一员。
> 在动手前必读：[`/ENGINEERING_GUIDE.md`](../../ENGINEERING_GUIDE.md) + [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)
> 重点章节：架构两层 Agent 设计 + 服务拓扑 + 机制 1（交易时段守卫）

---

## 服务定位（一句话）

**面向人类的代理**：所有 Web UI 请求的唯一入口，把 dashboard 的 HTTP / WebSocket 请求**转换成对 trading_agent 的 gRPC 调用 + Redis 状态聚合**。

⚠️ **不做业务决策**——决策都在 trading_agent。orchestrator 只做"协调 / 路由 / 聚合"。

---

## 两层 Agent 架构中的角色

```
人类（浏览器/手机）
       ↓
  dashboard_web (Next.js + Vision UI)
       ↓ HTTP REST + WebSocket
  ┌─────────────────────────────┐
  │ orchestrator                │ ← 你在这里
  │ (面向人类的代理)              │
  │ • REST + WebSocket           │
  │ • 聚合 Redis 状态             │
  │ • gRPC 客户端 → trading_agent │
  │ • 操作审计 + 交易时段守卫      │
  └─────────────────────────────┘
       ↓ gRPC :9090
  trading_agent (面向机器的代理)
       ↓ Broker
  CTP / 实盘
```

---

## 职责

### 做什么 ✅
- 暴露 REST API `:8080/api/v1/*` 供 dashboard_web 调用
- 暴露 WebSocket `:8080/ws` 推送实时状态（snapshot / delta / event）
- 通过 gRPC 客户端调用 trading_agent
- 聚合 Redis 状态（持仓 / 评分 / 服务注册等）
- 健康巡检（与 watchdog 互补，watchdog 做告警，orchestrator 给 UI 用）
- 操作审计：所有修改型操作记录到 InfluxDB `audit_log`
- 交易时段守卫：中间件拒绝交易时段的修改型操作（机制 1）

### 不做什么 ❌
- 不做交易决策
- 不直接操作 Broker
- 不持久化业务状态（**无状态服务**，重启不丢任何业务数据）
- 不直接访问 InfluxDB（除写 audit_log 经 db_writer 外）

---

## 内部结构

```
orchestrator/
├── Dockerfile
├── go.mod / go.sum
├── README.md                   ← 本文件
├── .env.example
├── cmd/main.go                 ← 入口：装配 + 启动 HTTP/WS
├── internal/
│   ├── domain/                 ← 业务逻辑（零外部依赖）
│   │   ├── aggregator.go         状态聚合纯逻辑
│   │   ├── command_router.go     UI 指令路由（一键平仓 / 暂停策略等）
│   │   ├── trading_calendar.go   ⭐ 交易日历（甲醇 4 时段 + 节假日）
│   │   └── audit.go              审计日志结构定义
│   ├── adapters/               ← 外部 IO
│   │   ├── trading_client/       ⭐ gRPC 客户端 → trading_agent
│   │   │   ├── client.go
│   │   │   └── stream.go         订阅状态变化流
│   │   ├── redis/                只读状态聚合（不写业务状态）
│   │   ├── audit_writer/         审计日志 → write:audit Stream
│   │   └── config/               Viper 配置
│   ├── api/                    ← 对外端点
│   │   ├── rest/                 ⭐ REST :8080/api/v1/*
│   │   │   ├── handlers.go         路由 handler
│   │   │   ├── middleware.go       ⭐ TradingSessionGuard 中间件
│   │   │   └── auth.go             认证（V2 加）
│   │   ├── websocket/            ⭐ WS :8080/ws
│   │   │   ├── hub.go              连接管理
│   │   │   ├── push_broker.go      消息分发
│   │   │   └── protocol.go         snapshot / delta / event 格式
│   │   ├── healthz.go            /healthz
│   │   └── metrics.go            /metrics
│   └── app/wire.go             ← 装配层
└── tests/{unit, integration}/
```

---

## 关键机制

### 1. 交易时段守卫（机制 1）⭐

中间件层拦截所有修改型操作：

```go
// internal/api/rest/middleware.go
func TradingSessionGuard(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !isMutation(r) {       // GET 等只读请求直接通过
            next.ServeHTTP(w, r)
            return
        }
        
        // 紧急平仓 = 例外（关乎安全）
        if r.URL.Path == "/api/v1/emergency_close" {
            next.ServeHTTP(w, r)
            return
        }
        
        if calendar.IsInTradingSession(time.Now()) {
            http.Error(w, fmt.Sprintf(
                "禁止在交易时段修改策略。下次可操作时间：%s",
                calendar.NextNonTradingWindow().Format("15:04")),
                http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

被守卫的操作：
- `POST /api/v1/strategies` 新增策略
- `PUT /api/v1/strategies/{id}` 编辑策略
- `DELETE /api/v1/strategies/{id}` 删除策略
- `POST /api/v1/strategies/{id}/promote` shadow → live
- `PUT /api/v1/risk/params` 改风控参数

不被守卫的操作（任何时间允许）：
- 所有 GET 查询
- `POST /api/v1/emergency_close` 紧急平仓
- `WebSocket /ws` 状态订阅

### 2. WebSocket 推送协议

三种消息类型：

```typescript
// snapshot - 客户端连接时一次性推送全量
{
  type: "snapshot",
  channel: "system_status",
  ts: 1718276400123,
  data: { /* 所有服务的状态 */ }
}

// delta - 状态变化时推送增量
{
  type: "delta",
  channel: "positions",
  ts: 1718276401000,
  data: { /* 变化的字段 */ }
}

// event - 关键事件触发
{
  type: "event",
  channel: "alerts",
  ts: 1718276402000,
  data: { event_type: "order_filled", ... }
}
```

### 3. 状态聚合

orchestrator 在内存维护全局状态快照：

```go
type GlobalState struct {
    Services    map[string]*HealthzResponse     // 9 个服务的健康度
    Positions   map[string]*Position            // 当前持仓
    AIScores    map[string]*AIScore             // 各合约评分
    Sentiments  map[string]*SentimentIndex      // 舆情指数
    TodayStats  *DailyStats                     // 今日报单 / 成交 / PnL
    SystemMode  string                          // Normal / Safe / Paused
}

// 聚合源
// - 巡检各服务 /healthz                  → Services
// - gRPC trading_agent.SubscribeStateChanges → Positions / Mode
// - Redis 订阅 ai_score:* / sentiment:*  → AIScores / Sentiments
```

---

## 对外接口

### REST API（:8080/api/v1/*）

#### 查询类（任何时间允许）

| Method | Path | 用途 |
|---|---|---|
| GET | `/overview` | 完整系统概览（首屏数据） |
| GET | `/services` | 各服务健康度详情 |
| GET | `/positions` | 当前持仓 |
| GET | `/orders` | 当日委托 |
| GET | `/trades` | 当日成交 |
| GET | `/ai/scores/current` | 当前 AI 评分 |
| GET | `/ai/scores/history?from=...&to=...` | 评分历史 |
| GET | `/strategies` | 策略列表（只读视图） |
| GET | `/risk/params` | 风控参数 |
| GET | `/audit?from=...` | 审计日志 |
| GET | `/audit/decisions/{id}` | 决策溯源（机制 6 反查） |
| GET | `/sentiment` | 舆情指数（P3） |

#### 修改类（**走交易时段守卫**）

| Method | Path | 用途 |
|---|---|---|
| POST | `/strategies` | 新增策略 |
| PUT | `/strategies/{id}` | 编辑策略 |
| DELETE | `/strategies/{id}` | 删除策略 |
| POST | `/strategies/{id}/promote` | shadow → live |
| POST | `/strategies/{id}/pause` | 暂停 |
| POST | `/strategies/{id}/resume` | 恢复 |
| PUT | `/risk/params` | 改风控参数 |

#### 紧急操作（**任何时间允许**）

| Method | Path | 用途 |
|---|---|---|
| POST | `/emergency_close` | ⭐ 一键紧急平仓 |

### WebSocket `:8080/ws`

```
客户端连接 → 服务器 push snapshot
            ↓
客户端 subscribe ["system_status", "positions", "scores", "alerts"]
            ↓
服务器 push delta + event 直到客户端断开
```

### gRPC 客户端 → trading_agent :9090

调用 `arbitrage.v1.TradingAgent` 服务：
- `GetSystemStatus`
- `EmergencyClose`
- `PauseStrategy / ResumeStrategy`
- `UpdateRiskParams`
- `SubscribeStateChanges` (流式)

---

## 写入的 Redis

| 目的 | Stream/Key | 内容 |
|---|---|---|
| 审计流 | `write:audit` (Stream) | 所有修改型操作日志 → db_writer |
| 服务注册 | `services:orchestrator` (KV) | TTL 30s |

⚠️ orchestrator **不写业务状态**到 Redis（写状态是 trading_agent / llm_scorer 等的事）。

---

## 消费的 Redis

仅做**只读聚合**：
- 各 `services:*` 看在线列表
- `ai_score:*` 看评分
- `sentiment:*` 看舆情
- `position:current` 看持仓快照（兜底，主路径走 gRPC）

---

## 依赖

| 依赖 | 类型 | 必需 |
|---|---|---|
| Redis 7.2 | 内部 | ✅ |
| trading_agent gRPC :9090 | 内部 | ✅ |
| `contracts/orchestrator.proto` | 内部契约 | ✅ |

---

## 配置（关键环境变量）

```bash
SERVICE_NAME=orchestrator
HTTP_PORT=8080

REDIS_ADDR=redis:6379
TRADING_AGENT_GRPC=trading_agent:9090

# 交易日历配置（默认甲醇）
TRADING_HOURS=09:00-10:15,10:30-11:30,13:30-15:00,21:00-23:00
HOLIDAYS_FILE=/app/config/holidays_2026.txt

# 操作审计
AUDIT_ENABLED=true

LOG_LEVEL=info
```

---

## 启动方式

```bash
# 本地（mock trading_agent）
go build -o orchestrator ./cmd
./orchestrator

# Docker
docker compose --profile ui up -d orchestrator
```

---

## 测试

```bash
# Unit 测试（mock gRPC + Redis）
go test ./internal/domain/...      # 交易日历 / 状态聚合纯逻辑
go test ./internal/api/...

# Integration 测试
docker compose -f docker-compose.test.yml up -d
go test -tags=integration ./tests/integration/...
```

---

## 已知限制 / 风险点

### 1. ⚠️ orchestrator 必须无状态
重启后从 trading_agent gRPC + Redis 重新聚合状态。**任何业务状态不存内存超过 30 秒**。

### 2. ⚠️ WebSocket 多客户端
手机 + Mac 同时连，每个连接独立 channel。snapshot 各自发，delta/event 广播。
推送频率限流：500ms 内多次相同 delta 合并为一条。

### 3. ⚠️ 交易日历必须每年初更新
`HOLIDAYS_FILE` 硬编码节假日，每年初手动更新（V2 计划对接交易所 API）。

### 4. ⚠️ 紧急操作必须二次确认
紧急平仓 API 必须前端做二次确认 dialog；后端再次校验请求源。

### 5. ⚠️ 不要把决策逻辑写进 orchestrator
反复强调：orchestrator 只做协调，决策在 trading_agent。
如果发现自己在写"是否买入"的逻辑 → 立刻停下，挪到 trading_agent。

---

## 与 ARCHITECTURE.md 的关联

- 「服务拓扑」：编排层
- 「两层 agent 关键设计哲学」：面向人类的代理
- 机制 1：交易时段守卫 + 灰度发布
- 机制 6：决策溯源 API

---

## AI Agent 准则

- ✅ 加新 REST endpoint → 同步更新 dashboard_web 的 API 客户端
- ✅ 改 gRPC 调用 → 同时更新 `contracts/orchestrator.proto`
- ✅ 加 WebSocket channel → 文档化推送时机
- ❌ **永远不要**在 orchestrator 里写业务决策（"如果价格 > X 就买"）
- ❌ 不要在 `domain/` 里 import grpc / redis / http
- ❌ 不要在 orchestrator 里持久化业务状态
- ❌ 不要绕过 TradingSessionGuard 中间件

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版（v2.2 拆出 dashboard 后） |
