# services/db_writer — InfluxDB 串行写入服务

> 本服务属于 arbitrage-system 微服务集合的一员。
> 在动手前必读：[`/ENGINEERING_GUIDE.md`](../../ENGINEERING_GUIDE.md) + [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)
> 重点章节：架构机制 2（DB 写入串行化）+ 机制 6（数据持久化）

---

## 服务定位（一句话）

**全系统唯一的 InfluxDB 写入入口**。所有微服务 → Redis Streams → 本服务 → InfluxDB（CQRS 写侧）。

---

## 为什么需要这个服务（机制 2）

### 之前的问题
如果每个服务直接写 InfluxDB：
- 多个连接抢锁，吞吐下降
- InfluxDB 挂掉 → 业务卡死（因为业务在等写入响应）
- 重试逻辑要在每个服务里写一遍
- 没有统一的失败兜底

### 现在的设计
- 业务服务只 XADD 到 Redis Stream（毫秒级，永远不阻塞）
- db_writer 单实例消费，批量写入 InfluxDB
- 失败 → 重试 → 死信队列 → 钉钉告警
- 业务零阻塞、写入有序、重试集中、监控统一

---

## 职责

### 做什么 ✅
- 消费 6 个 Redis Streams（`write:*`）作为写入队列
- 按消费组（XREADGROUP）单实例消费
- 批量聚合：100 条 / 1 秒触发一次 InfluxDB 写入
- 失败重试：3 次指数退避（1s / 4s / 16s）
- 失败转死信队列：`dlq:write:<bucket>`
- 死信队列有数据 → 推钉钉告警
- 暴露写入指标（吞吐 / 延迟 / 失败率）到 /metrics

### 不做什么 ❌
- 不做业务逻辑（纯持久化通道）
- 不修改写入内容（透明转发）
- 不读 InfluxDB（纯写）

---

## ⚠️ 单实例约束

**必须**只跑一个 db_writer 实例：
- docker-compose 里固定 `replicas: 1`
- 用 Redis 消费组（XREADGROUP）确保单消费者语义
- 多实例会破坏写入顺序

如果未来想扩容（比如分合约分流），需要按 symbol 分 stream，每个 stream 单消费者。

---

## 内部结构

```
db_writer/
├── Dockerfile
├── go.mod / go.sum
├── README.md                   ← 本文件
├── .env.example
├── cmd/main.go                 ← 入口：启动消费循环
├── internal/
│   ├── domain/                 ← 业务逻辑（零外部依赖）
│   │   ├── batch_aggregator.go    100 条 / 1s 聚合规则
│   │   ├── retry_policy.go        重试策略（指数退避）
│   │   └── dlq_handler.go         死信处理逻辑
│   ├── adapters/               ← 外部 IO
│   │   ├── redis/                Stream 消费 + 死信队列
│   │   │   ├── consumer.go         XREADGROUP 单消费者
│   │   │   └── dlq.go              死信队列写入
│   │   ├── influxdb/             批量写入
│   │   │   └── writer.go           InfluxDB Line Protocol
│   │   ├── alert/                钉钉告警
│   │   └── config/
│   └── api/
│       ├── healthz.go              /healthz
│       └── metrics.go              /metrics
└── tests/{unit, integration}/
```

---

## 消费的 Redis Streams

| Stream | 生产者 | 写入 InfluxDB Bucket |
|---|---|---|
| `write:tick` | ctp_tick | `ma_tick` |
| `write:indicator` | trading_agent | `indicator` |
| `write:score` | llm_scorer | `ai_score_history` |
| `write:llm_analysis` | llm_scorer | `llm_analysis` |
| `write:order` | trading_agent | `order_history` |
| `write:position_snapshot` | trading_agent | `position_snapshots` |
| `write:sentiment` | intel_collector | `raw_intel` / `parsed_intel` / `cleaned_intel`（三层） |
| `write:audit` | orchestrator | `audit_log` |
| `write:trade_decisions` | trading_agent | `trade_decisions`（机制 6） |
| `write:sentiment_index` | llm_scorer | `sentiment_index` |

每个 Stream 用独立的消费组：
```
XGROUP CREATE write:tick db_writer_group $ MKSTREAM
XGROUP CREATE write:indicator db_writer_group $ MKSTREAM
...
```

---

## 数据流

```
业务服务（ctp_tick / trading_agent / llm_scorer / ...）
   │ XADD write:<bucket> <data>
   ▼
Redis Stream（持久化、有序、可重放）
   │ XREADGROUP db_writer_group consumer1
   ▼
db_writer 内存缓冲（按 bucket 分类）
   │ batch_size >= 100 OR elapsed >= 1s
   ▼
InfluxDB 批量写入（Line Protocol）
   │ 成功 → XACK
   │ 失败 → 重试 3 次 → DLQ
   ▼
失败 → XADD dlq:write:<bucket>
       钉钉告警
```

---

## 失败处理

### 重试策略

```go
// internal/domain/retry_policy.go
const (
    MaxRetries = 3
    InitialBackoff = 1 * time.Second
    BackoffMultiplier = 4
)

// 1s → 4s → 16s
```

### 死信队列

每个 `write:*` 对应一个 `dlq:write:*`：
- 死信数据保留**7 天**（自动 EXPIRE）
- 死信队列长度 > 0 → 立即推钉钉
- 修复后可手动 `XREAD dlq:write:tick` 重新写入

### 监控指标（/metrics）

```
db_writer_write_total{bucket="ma_tick",status="success"}
db_writer_write_total{bucket="ma_tick",status="failure"}
db_writer_dlq_size{bucket="ma_tick"}
db_writer_batch_latency_seconds_bucket{bucket="ma_tick",le="0.1"}
db_writer_pending_messages{stream="write:tick"}
```

---

## 对外接口

### 暴露的端点

| 类型 | 端口 | 路径 | 用途 |
|---|---|---|---|
| HTTP | 8081 | `/healthz` | 健康检查（含 InfluxDB 连通性 + DLQ 长度） |
| HTTP | 8081 | `/metrics` | Prometheus 指标 |

### 写入的 Redis

| 目的 | Key | 用途 |
|---|---|---|
| 死信队列 | `dlq:write:<bucket>` (Stream) | 失败数据 |
| 服务注册 | `services:db_writer` (KV) | TTL 30s |

---

## 依赖

| 依赖 | 类型 | 必需 |
|---|---|---|
| Redis 7.2 | 内部 | ✅ |
| InfluxDB 2.7 | 内部 | ✅ |

---

## 配置（关键环境变量）

```bash
SERVICE_NAME=db_writer
HEALTH_PORT=8081

REDIS_ADDR=redis:6379
INFLUXDB_URL=http://influxdb:8086
INFLUXDB_TOKEN=xxx
INFLUXDB_ORG=arbitrage

# 批量写入参数
BATCH_SIZE=100
BATCH_INTERVAL=1s

# 重试参数
MAX_RETRIES=3
INITIAL_BACKOFF=1s

# 死信告警
DLQ_ALERT_THRESHOLD=10           # 死信数 > 10 推钉钉
DLQ_TTL=168h                      # 7 天

LOG_LEVEL=info
```

---

## 启动方式

```bash
# Docker（生产）
docker compose --profile full up -d db_writer
docker logs -f db_writer
```

⚠️ **不要在本地开发模式跑多个实例**——会破坏单消费者语义。

---

## 测试

```bash
# Unit 测试（mock Redis + InfluxDB）
go test ./internal/domain/...

# Integration 测试
docker compose -f docker-compose.test.yml up -d redis influxdb
go test -tags=integration ./tests/integration/...
```

---

## 已知限制 / 风险点

### 1. 🔴 单实例约束不可破坏
多实例运行会导致消息重复写 InfluxDB / 顺序错乱。
docker-compose 必须 `replicas: 1`。

### 2. ⚠️ Stream 长度监控
如果 db_writer 跟不上写入速度，Stream 会堆积：
- 监控 `pending_messages` 指标
- 超过 10000 推钉钉告警
- 考虑加批量大小或开启 InfluxDB 分片

### 3. ⚠️ 死信队列不能忽略
DLQ 有数据 = 真实数据丢失风险。
必须人工介入处理。

### 4. ⚠️ InfluxDB 必须先建好 bucket
db_writer 启动时**不会自动建 bucket**，需要 `init_redis.sh` 或手动建好。

### 5. ⚠️ 写入失败不影响业务
业务服务只 XADD 到 Redis，不等响应。
所以 db_writer 即使全挂，业务还能跑（数据先囤在 Redis）。
但 Redis 内存有限（1G），长期不写会 OOM。

---

## 与 ARCHITECTURE.md 的关联

- 机制 2：DB 写入串行化（核心服务）
- 机制 3：数据可靠性（Redis 持久化 + 关键数据双写 InfluxDB）
- 机制 6：数据清洗管道与可追溯性（5 阶段全部经本服务持久化）

---

## AI Agent 准则

- ✅ 加新 Bucket → 在 InfluxDB 里建 + 在本文档登记
- ✅ 改批量参数 → 测试性能影响
- ❌ **绝对不要**让 db_writer 改写数据（必须透明转发）
- ❌ 不要在 db_writer 里加业务逻辑（纯持久化通道）
- ❌ 不要破坏单实例约束
- ❌ 不要忽视死信队列告警

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版（v2.0 引入） |
