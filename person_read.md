# 项目代码阅读路线

> 个人审查笔记，AI agent 不强制阅读。

## 代码全景

```
总代码量: ~17,000 行（不含文档/生成代码）

trading_agent  4003 Go   ████████████████████  决策大脑
ctp_tick       3523 Go   ██████████████████    行情采集
llm_scorer     2774 Py   ██████████████        AI 评分
orchestrator   2540 Go   █████████████         编排层
db_writer      1517 Go   ████████             持久化网关
watchdog        577 Go   ███                   监控
dashboard_web   421      ██                   前端
intel_collector 125 Py   █                    舆情（占位）
pkg/           1682 Go   ████████             共享库
contracts/      339 proto                      契约 SSOT
```

---

## 第 1 站：契约层（30 分钟）

先看懂跨服务通信的"语言"——所有服务间通信只走这些 proto。

```
contracts/
├── orchestrator.proto   ← orchestrator ↔ trading_agent gRPC
├── ai_score.proto       ← AI 评分数据结构（score/confidence/horizon/drivers）
├── tick.proto           ← Tick 行情字段
├── order.proto          ← 报单（开/平/止盈止损）
├── position.proto       ← 持仓状态
├── indicator.proto      ← 技术指标（MA/Boll/ATR 等）
├── healthz.proto        ← 统一健康检查响应
└── intel_event.proto    ← 舆情事件格式
```

---

## 第 2 站：共享基础设施（20 分钟）

所有 Go 服务都依赖 `pkg/`，读一次就够。

```
pkg/
├── redis/
│   ├── adapter.go       ← 统一 Redis client（规则 7：禁止各服务自己 new）
│   └── retry.go         ← 指数退避重试（XAddWithRetry / GetWithRetry）
└── registry/
    └── registry.go      ← 服务注册 + 心跳续约（Go 端）
```

---

## 第 3 站：核心数据链路（2 小时）⭐ 最重要

按一笔 Tick 从产生到报单的完整路径走：

### 3a. 行情入口 — ctp_tick/
```
internal/
├── domain/              ← Tick 标准化纯逻辑（raw → parsed）
├── adapters/
│   ├── ctp/md.go        ← CTP 行情订阅（cgo 封装 openctp）
│   ├── ctp/td.go        ← CTP 交易接口（查合约元数据）
│   └── redis/           ← 写 tick:MA2509 Stream + write:tick Stream
└── api/                 ← /healthz + /metrics + /internal/snapshot
```

### 3b. 决策核心 — trading_agent/ ⭐ 最复杂
```
internal/
├── domain/
│   ├── indicator/       ← MA / 布林带 / ATR / 量比（纯算法）
│   ├── strategy/        ← 策略引擎（V1: MA 金叉死叉）
│   ├── decision/        ← ⭐ 决策融合公式：finalBias = score×conf×decay×alpha
│   ├── risk/            ← 风控闸门（单笔止损/日亏损上限/仓位上限）
│   └── position/        ← 持仓管理 + SafeMode
├── adapters/
│   ├── broker/          ← ⭐⭐ Broker 抽象层（精髓！）
│   │   ├── interface.go ← Broker interface 定义
│   │   └── ctp/         ← CTP 报单实现（V1 唯一）
│   ├── redis/           ← 消费 tick:MA2509 + 读写状态
│   └── influxdb/        ← 启动对账时直读（平时走 db_writer）
├── api/
│   ├── grpc/            ← gRPC :9090（暴露给 orchestrator）
│   ├── healthz.go       ← :8081/healthz
│   └── metrics.go       ← :8081/metrics
└── app/wire.go          ← 装配层（DI）
```

**关键设计**：`broker/interface.go` 定义了 `Subscribe / PlaceOrder / CancelOrder / QueryPositions` 等接口。V1 只实现 CTP，V2 加股票时只需新增 `qmt/broker.go`，策略层零改动。

### 3c. 持久化终点 — db_writer/
```
internal/
├── domain/              ← 批量聚合规则（100 条或 1 秒触发 flush）
├── adapters/
│   ├── redis/           ← 消费 write:* Streams（单实例防并发）
│   ├── influxdb/        ← 批量写入 InfluxDB
│   └── dlq/             ← 死信队列（写入失败 → 重试 3 次 → 死信）
└── api/                 ← /healthz + /metrics
```

**关键设计**：CQRS 写侧，业务服务只 XADD（毫秒级），不等 InfluxDB 响应。db_writer 单实例串行消费，保证写入顺序。

---

## 第 4 站：AI 层（1 小时）

### llm_scorer/
```
adapters/
├── llm_gateway.py       ← 多 provider 网关（DeepSeek→GLM→Mimo 二级熔断）
├── config.py            ← 环境变量自动发现 provider
├── data_fetcher.py      ← 从 Redis 获取行情/指标/新闻
├── redis_writer.py      ← 写 ai_score:* KV + write:score Stream
└── registry.py          ← 服务注册 + 独立心跳循环

domain/agents/
├── base.py              ← Agent 基类 + 模板方法 + 共享 JSON 解析
├── sentiment_agent.py   ← 舆情分析（输入=新闻列表，输出=方向分+置信度）
├── technical_agent.py   ← 技术面分析（MA/Boll/ATR/量比）
├── fundamental_agent.py ← 基本面分析（库存/基差/开工率/原油）
├── aggregator.py        ← 汇总 + 交叉验证 + 分歧检测
├── runner.py            ← asyncio.gather 并行编排 + 45s 超时降级
├── guardrails.py        ← 交易时段校验 + symbol 白名单 + 输出校验
└── tools.py             ← Agent 工具函数（stub → 待 Redis 接入）

main.py                  ← 入口：评分循环(FastAPI + asyncio)
```

---

## 第 5 站：编排层 + 监控（30 分钟）

### orchestrator/
```
internal/
├── domain/
│   ├── aggregator.go        ← 状态聚合（Redis + gRPC → 统一视图）
│   ├── trading_calendar.go  ← 交易日历 + 交易时段守卫
│   └── command_router.go    ← UI 指令路由（一键平仓 / 暂停策略）
├── adapters/
│   ├── trading_client/      ← gRPC 客户端 → trading_agent
│   ├── redis/               ← 只读状态聚合
│   └── audit_writer/        ← 操作审计日志
└── api/
    ├── rest/                ← REST :8080/api/v1/*（给 dashboard）
    │   └── middleware.go     ← ⭐ TradingSessionGuard（交易时段禁止修改操作）
    ├── websocket/           ← WS :8080/ws（实时推送 snapshot/delta/event）
    ├── healthz.go
    └── metrics.go
```

**关键设计**：orchestrator 是"面向人类的代理"，不做交易决策。所有修改型操作走 `TradingSessionGuard` 中间件，紧急平仓例外。

### watchdog/
```
internal/
├── domain/              ← 健康判定规则 + 内存三级告警（80%/90%/95%）
└── adapters/
    ├── healthz_client.go    ← 轮询各服务 /healthz
    ├── disk_monitor.go      ← 磁盘使用率
    ├── memory_monitor.go    ← 内存使用率
    └── dingtalk.go          ← 钉钉 Webhook 推送
```

---

## 建议阅读顺序

```
① contracts/*.proto          30min  搞懂"语言"
② pkg/redis/ + pkg/registry  20min  共享基础设施
③ ctp_tick → trading_agent → db_writer  2h  ⭐ 核心链路
④ llm_scorer                 1h    AI 评分
⑤ orchestrator + watchdog    30min 编排 + 监控
```

总共约 **4 小时**读完核心代码。

---

## 关键文件速查

| 文件 | 内容 | 为什么重要 |
|------|------|-----------|
| `services/trading_agent/internal/adapters/broker/interface.go` | Broker 抽象 | 精髓：V2 加股票只加实现 |
| `services/trading_agent/internal/domain/decision/fuser.go` | 决策融合 | finalBias = score×conf×decay×alpha |
| `services/orchestrator/internal/api/rest/middleware.go` | 时段守卫 | 交易时段禁止修改操作 |
| `services/db_writer/internal/adapters/redis/consumer.go` | 串行消费 | CQRS 写侧 |
| `services/llm_scorer/adapters/llm_gateway.py` | 二级熔断 | DeepSeek→GLM→Mimo |
| `services/llm_scorer/domain/agents/runner.py` | 并行编排 | N Agent asyncio.gather |
| `contracts/orchestrator.proto` | gRPC 契约 | orchestrator ↔ trading_agent |
| `pkg/redis/adapter.go` | 统一 Redis | 规则 7 的唯一入口 |

---

## QUESTION

1. RegisterAndKeepAlive 只有watchdog 和 tradeagent在用，其他节点怎么注册的，
比如ctp_tick 直接使用的 func Register(ctx context.Context, client pkgredis.RedisClient, info ServiceInfo, opts ...Option) (*Registry, error)，为啥
2. ctp_tick 里面的内存快照是啥
3. func (w *TickWriter) Send(symbol string, tick *domain.StandardTick) bool 这里面是怎么发到管道的；ctp_tick 是如何写数据到db的


sk-fuyao-8rKHlwUwA2cvGJDwJ-DhPHTa-iPM7lQx