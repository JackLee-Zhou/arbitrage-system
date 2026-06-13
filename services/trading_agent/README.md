# services/trading_agent — 交易决策核心

> 本服务属于 arbitrage-system 微服务集合的一员。
> 在动手前必读：[`/ENGINEERING_GUIDE.md`](../../ENGINEERING_GUIDE.md) + [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)
> 重点章节：架构机制 1-4 + 机制 6（决策溯源）

---

## 服务定位（一句话）

**整个系统的决策大脑**。订阅行情 → 算指标 → 跑策略 → 融合 AI 评分 → 风控 → 报单 → 持仓管理。

⚠️ **这是真金白银的关键服务**。任何修改都要谨慎、有充分测试覆盖。

---

## 职责

### 做什么 ✅
- 订阅 Redis Stream `tick:MA2509` 接收行情
- 计算技术指标（MA / 布林带 / ATR / 量比）
- 运行策略引擎（V1：MA 金叉死叉）
- 决策融合：`finalBias = score × confidence × decay × alpha`
- AI 评分老化（< 15min/15-30min/>30min decay 1.0/0.5/0.0）
- 风控闸门（单笔止损 / 日最大亏损 / 仓位限制 / 断线保护）
- 通过 Broker interface 报单（V1: CTP；V2: QMT 等）
- 持仓管理 + SafeMode（启动对账失败时只平仓不开新仓）
- 检测剧烈波动 → 调 llm_scorer `/emergency_analysis`
- 暴露 gRPC :9090 给 orchestrator 查询和发指令

### 不做什么 ❌
- 不抓行情（ctp_tick 的事）
- 不调 LLM（llm_scorer 的事）
- 不抓舆情（intel_collector 的事）
- 不直接面向用户/UI（orchestrator 的事）
- 不直接写 InfluxDB（走 db_writer，唯一例外：CTP 查询持仓时直接读 InfluxDB 兜底）

---

## v2.1 合并：含 indicator 模块

原 `indicator` 服务合并入 `trading_agent.domain.indicator`。
理由：生命周期完全绑定 + 减少 Redis 跳数 + 降低延迟。

→ `domain/indicator/` 现在是 trading_agent 的内部包，**不暴露给其他服务**。

如果以后真有"多策略共享同一指标"需求，再拆出来。

---

## 内部结构

```
trading_agent/
├── Dockerfile                  ← cgo 编译 CTP + 静态二进制
├── go.mod / go.sum
├── README.md                   ← 本文件
├── .env.example
├── cmd/main.go                 ← 入口：启动重连循环 + 三步对账
├── internal/
│   ├── domain/                 ⭐ 业务逻辑（零外部依赖）
│   │   ├── indicator/          技术指标（v2.1 合并入）
│   │   │   ├── ma.go             移动均线
│   │   │   ├── boll.go           布林带
│   │   │   ├── atr.go            ATR
│   │   │   └── volume.go         量比
│   │   ├── strategy/           策略引擎
│   │   │   ├── engine.go         调度
│   │   │   └── ma_trend.go       V1 唯一策略：MA 金叉死叉
│   │   ├── decision/           决策融合
│   │   │   ├── fuser.go          ⭐ finalBias 计算
│   │   │   └── decay.go          ⭐ AI 评分老化（硬编码）
│   │   ├── risk/               风控
│   │   │   ├── manager.go        风控规则总入口
│   │   │   ├── single_loss.go    单笔止损
│   │   │   ├── daily_limit.go    日最大亏损
│   │   │   └── position_limit.go 仓位上限
│   │   └── position/           持仓管理
│   │       └── manager.go
│   ├── adapters/               ⭐ 外部 IO 适配
│   │   ├── broker/             ⭐⭐ Broker 抽象层
│   │   │   ├── interface.go      Broker interface 定义
│   │   │   ├── ctp/              CTP 实现（V1 唯一）
│   │   │   │   ├── md.go         订阅
│   │   │   │   ├── td.go         报单/撤单/查持仓
│   │   │   │   └── types.go      CTP ↔ contract 转换
│   │   │   └── (qmt/, easytrader/  V2 加)
│   │   ├── redis/              订阅 tick + 状态读写
│   │   ├── influxdb/           订单流水持久化（仅启动对账时直读）
│   │   └── llm_client/         调 llm_scorer /emergency_analysis
│   ├── api/                    ← 对外端点
│   │   ├── grpc/               ⭐ gRPC :9090（暴露给 orchestrator）
│   │   │   ├── server.go         gRPC 服务实现
│   │   │   └── handlers.go       请求处理器
│   │   ├── healthz.go          :8081/healthz
│   │   └── metrics.go          :8081/metrics
│   └── app/wire.go             ⭐ 装配层（DI）
└── tests/{unit, integration}/
```

---

## 关键机制

### 1. 三步启动对账（机制 3）⭐⭐⭐

```go
// cmd/main.go 启动流程
func main() {
    // ... 装配 ...
    
    // ⭐ 三步对账
    state := State{}
    
    // Step 1: Redis 加载
    state.Position = redis.Get("position:current")
    
    // Step 2: InfluxDB 补齐最近 1h（如果 Redis 没有）
    if state.Position == nil {
        state.Position = influx.QueryLatestPositionSnapshot()
    }
    
    // Step 3: ⭐ CTP 查询真实持仓 → 对账
    ctpPosition := ctp.QueryAccountPosition(timeout=30s)
    if !state.Position.Equals(ctpPosition) {
        // 不一致：进 SafeMode
        state.Mode = ModeSafe
        publishAlert(...)  // 推钉钉
    }
    
    runEngine(state)
}
```

**核心原则**：CTP 查询接口是**黄金真相**，本地状态只是缓存。

### 2. SafeMode（持仓对账失败时）

```
SafeMode 行为：
  ✅ 允许平仓
  ❌ 禁止开新仓
  推钉钉持续告警
  人工介入后手动切回 Normal
```

### 3. 决策融合公式（V1 简化版）

```
finalBias    = score × confidence × decay × alpha
positionSize = technicalSignal.strength × (1 + finalBias)

其中：
  alpha = 0.2          (V1 轻权重，max ±20% 仓位影响)
  decay = 1.0 / 0.5 / 0.0
    时间老化硬编码：
      < 15min:  decay = 1.0  (full 信任)
      15-30min: decay = 0.5  (权重减半)
      > 30min:  decay = 0.0  (盲打模式 + 钉钉告警)
```

### 4. 紧急触发（机制 4）

```go
// 每接收 Tick 后检测剧烈波动
func (e *Engine) onTick(tick Tick) {
    volatility := e.calcVolatility(tick, lastN=60)
    
    if volatility.PriceChangePct > 1.0 ||
       volatility.VolumeMultiplier > 3.0 ||
       volatility.OneSidedRatio > 0.85 {
        
        // 5 分钟防抖
        if time.Since(e.lastEmergencyAt) < 5*time.Minute { return }
        e.lastEmergencyAt = time.Now()
        
        // 异步触发紧急分析（不阻塞主流程）
        go e.llmClient.RequestEmergency(...)
    }
    
    // 检查 emergency 评分
    if emergencyScore := redis.Get("ai_score:MA2509:emergency"); emergencyScore != nil {
        e.enterCautiousMode(emergencyScore.suggested_action)
    }
}
```

### 5. Broker 抽象层（V2 加股票预留）

```go
// internal/adapters/broker/interface.go
type Broker interface {
    Subscribe(symbols []string) error
    PlaceOrder(req OrderRequest) (OrderID, error)
    CancelOrder(id OrderID) error
    QueryPositions() ([]Position, error)
    OnTick(handler func(Tick))
    OnOrderUpdate(handler func(OrderEvent))
}

// V1 实现：ctp/broker.go
// V2 加：qmt/broker.go, easytrader/broker.go
// ⭐ 业务层（domain/strategy/）通过 interface 调用，永远不直接依赖具体 broker
```

---

## 对外接口

### 暴露的端点

| 类型 | 端口 | 路径 | 用途 |
|---|---|---|---|
| gRPC | 9090 | `arbitrage.v1.TradingAgent/*` | ⭐ 仅供 orchestrator 调用 |
| HTTP | 8081 | `/healthz` | 健康检查 |
| HTTP | 8081 | `/metrics` | Prometheus 指标 |

### gRPC 主要方法（contracts/orchestrator.proto）

```
service TradingAgent {
  rpc GetSystemStatus(...)         返回当前持仓 / PnL / 模式（Normal/Safe）
  rpc EmergencyClose(...)          一键平仓
  rpc PauseStrategy(...)           暂停策略（不影响已有持仓平仓）
  rpc ResumeStrategy(...)          恢复策略
  rpc UpdateRiskParams(...)        修改风控参数（走交易时段守卫）
  rpc SubscribeStateChanges(...)   流式推送状态变化（gRPC stream）
}
```

### 写入的 Redis

| 目的 | Key/Stream | 用途 |
|---|---|---|
| 持仓快照 | `position:current` (KV) | 实时持仓状态 |
| 引擎状态 | `engine:status` (KV) | Normal/Safe/Paused |
| 信号流 | `signal:MA2509` (Stream) | 策略产生的买卖信号 |
| 报单流 | `write:order` (Stream) | 让 db_writer 写 InfluxDB |
| 持仓快照流 | `write:position_snapshot` (Stream) | 同上 |
| 服务注册 | `services:trading_agent` (KV) | TTL 30s |

### 消费的 Redis

| 来源 | 行为 |
|---|---|
| `tick:MA2509` (Stream) | 订阅行情 |
| `ai_score:MA2509:live` (KV) | 5 秒轮询 AI 评分 |
| `ai_score:MA2509:emergency` (KV) | 紧急评分（触发谨慎模式） |
| `sentiment:MA2509:current` (KV) | P3 加：舆情指数纳入决策 |

---

## 依赖

| 依赖 | 类型 | 必需 |
|---|---|---|
| Redis 7.2 | 内部 | ✅ |
| InfluxDB | 内部 | ✅（启动对账时直读，正常运行经 db_writer） |
| llm_scorer | 内部 | ✅（紧急分析端点） |
| CTP 前置机 | 外部 | ✅ |
| `contracts/*.proto` 全部 | 内部契约 | ✅ |

---

## 配置（关键环境变量）

```bash
SERVICE_NAME=trading_agent
HEALTH_PORT=8081
GRPC_PORT=9090

REDIS_ADDR=redis:6379
INFLUXDB_URL=http://influxdb:8086
INFLUXDB_TOKEN=xxx
LLM_SCORER_URL=http://llm_scorer:5000

# CTP（同 ctp_tick）
CTP_FRONT_ADDR=...
CTP_BROKER_ID=...
CTP_USER_ID=...
CTP_PASSWORD=...

# 业务参数
SYMBOL=MA2509
ALPHA=0.2                       # AI 评分权重
ALPHA_BETA_SENTIMENT=0.0        # P3 舆情权重，P1=0
INITIAL_CAPITAL=100000          # 初始资金（人民币）

# 风控
MAX_POSITION=2                  # 最大持仓手数
DAILY_MAX_LOSS_PCT=0.02         # 日最大亏损 2%
SINGLE_STOP_LOSS_PCT=0.005      # 单笔止损 0.5%

LOG_LEVEL=info
```

---

## 启动方式

```bash
# 本地（mock CTP + Redis）
go build -tags="!ctp" -o trading_agent ./cmd
./trading_agent

# Docker（真 CTP）
docker compose --profile futures up -d trading_agent
docker logs -f trading_agent
```

---

## 测试

### Unit 测试（必须 ≥ 70% domain/ 覆盖率）

```bash
go test ./internal/domain/...
# domain 测试不依赖 Redis/CTP/HTTP，必须毫秒级
```

### Integration 测试

```bash
docker compose -f docker-compose.test.yml up -d redis influxdb
go test -tags=integration ./tests/integration/...
```

---

## 已知限制 / 风险点

### 1. 🔴 这是个「真金白银」的服务
代码改动必须有 unit 测试 + 谨慎 review。

### 2. ⚠️ 三步对账必须严格执行
启动时 CTP 查询失败不能跳过——直接进 SafeMode。

### 3. ⚠️ 防抖必须严格
紧急触发 5 分钟防抖、报单防止重复发送（订单引用号唯一）。

### 4. ⚠️ AI 评分老化是硬规则
`decay` 函数在代码里硬编码，**不要做成配置**——防止"忘了关"导致裸奔。

### 5. ⚠️ 仓位限制不可绕过
风控闸门是最后防线，**不允许策略层"特殊情况下绕过风控"**。

### 6. ⚠️ CTP 字段精度
价格用 `int64` 存储（最小变动单位），不用 `float64` 防精度损失。

---

## 与 ARCHITECTURE.md 的关联

- 机制 1：策略管理 + 交易时段守卫
- 机制 2：写入串行化（trading_agent 不直接写 InfluxDB）
- 机制 3：数据可靠性（三步对账 + SafeMode）
- 机制 4：事件驱动 AI 紧急触发
- 机制 6：决策溯源（写 trade_decisions bucket）
- Broker 抽象层：V2 股票接入预留

---

## AI Agent 准则

- ✅ 改 `domain/decision/` 或 `domain/risk/` → **必须** unit 测试覆盖
- ✅ 改 Broker interface → 同步更新 `contracts/order.proto` + 所有 broker 实现
- ❌ 不要在 `domain/` 里 import redis / grpc / cgo
- ❌ 不要在策略层"绕过"风控闸门
- ❌ 不要把 AI 评分老化做成可配置（硬编码就是硬编码）
- ❌ 不要绕过 db_writer 直接写 InfluxDB（除启动对账外）
- ❌ 不要修改 alpha 默认值 0.2（这是 ARCHITECTURE.md 硬决策）

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版（v2.1 合并 indicator） |
