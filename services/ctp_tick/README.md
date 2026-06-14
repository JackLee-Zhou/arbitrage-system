# services/ctp_tick — CTP 行情订阅服务

> 本服务属于 arbitrage-system 微服务集合的一员。
> 在动手前必读：[`/ENGINEERING_GUIDE.md`](../../ENGINEERING_GUIDE.md) + [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)

---

## 服务定位（一句话）

启动时通过 TD API 查询**全市场期货合约**并全部订阅。所有合约 Tick → `write:tick` 异步批量落库；用户关注合约（SUBSCRIBE_SYMBOLS）额外 → `tick:<symbol>` 近实时推送分析层。

---

## 职责

### 做什么 ✅
- 启动时通过 **TD API** 查询六大交易所全市场期货合约（ProductClass='1'）
- 通过 **MD API** 订阅全市场 ~330 个期货合约
- 接收 `OnRtnDepthMarketData` 回调，**非阻塞**发送到异步写入管道（buffer 10000）
- 全部合约：批量攒批（100 条/1 秒）→ `write:tick` → db_writer 落库
- 关注合约（SUBSCRIBE_SYMBOLS）：逐笔近实时 → `tick:<symbol>` → trading_agent 消费
- 标准化为 protobuf `Tick` 消息（domain/normalizer.go）
- 自动重连：滑动窗口限频（60s / 3 次），指数退避（10s/20s/30s）

### 不做什么 ❌
- 不算技术指标（那是 trading_agent.domain.indicator 的事）
- 不做交易决策
- **不直接写 InfluxDB**（统一走 db_writer 串行化）
- 不报单（那是 trading_agent.adapters.broker 的事）
- **CTP 回调线程不阻塞**（非阻塞 channel send，满则丢弃）

---

## 内部结构

```
ctp_tick/
├── Dockerfile                  ← 多阶段构建：cgo 编译 CTP + 静态二进制
├── go.mod / go.sum
├── README.md                   ← 本文件
├── .env.example
├── cmd/main.go                 ← 入口：装配 + 启动
├── internal/
│   ├── domain/                 ← 业务逻辑（零外部依赖）
│   │   └── normalizer.go         CTP → Tick 标准化 + 安全过滤
│   ├── adapters/               ← 外部 IO
│   │   ├── ctp/                  cgo 封装 openctp/ctp-go
│   │   │   ├── md.go               行情接口（MD API）
│   │   │   ├── md_mock.go          macOS mock 行情（//go:build !ctp）
│   │   │   ├── td.go               ⭐ TD API 全市场合约查询（//go:build ctp）
│   │   │   ├── td_mock.go          Mock 合约列表（//go:build !ctp）
│   │   │   ├── spi.go              CTP 回调实现
│   │   │   ├── reconnect.go        断线重连逻辑
│   │   │   └── types.go            CTP 类型 + 接口定义
│   │   ├── redis/                ⭐ 异步写入管道
│   │   │   └── writer.go           TickWriter（batch 100条/1s + 逐笔 focus）
│   │   └── config/               Viper 配置加载
│   └── api/                    ← 对外端点
│       ├── healthz.go            GET /healthz
│       └── metrics.go            GET /metrics
└── tests/
    ├── unit/                   ← 不依赖 CTP/Redis（用 mock）
    └── integration/            ← 用 SimNow + 真 Redis
```

### 关键文件职责

| 文件 | 职责 |
|---|---|
| `cmd/main.go` | DI 装配：TD 合约查询 → MD 全量订阅 → 异步回调注册 |
| `domain/normalizer.go` | CTP → StandardTick → ProtoTick 纯函数转换 |
| `adapters/ctp/md.go` | MD API：注册前置机 → 登录 → 全量订阅 |
| `adapters/ctp/td.go` | TD API：查询 6 交易所 → 过滤期货合约 |
| `adapters/ctp/spi.go` | CThostFtdcMdSpi 回调：OnFrontConnected / OnRtnDepthMarketData |
| `adapters/ctp/reconnect.go` | 断线重连：滑动窗口（60s / 3 次）+ 指数退避 |
| `adapters/ctp/types.go` | 类型定义 + MarketDataSubscriber / ContractQuerier 接口 |
| `adapters/redis/writer.go` | TickWriter：非阻塞 Send → writerLoop（batch 100/1s + focus 逐笔） |

---

## 对外接口

### 暴露的端点

| 类型 | 端口 | 路径 | 用途 |
|---|---|---|---|
| HTTP | 8081 | `/healthz` | 健康检查（含 CTP 连接状态） |
| HTTP | 8081 | `/metrics` | Prometheus 指标 |
| HTTP | 8081 | `/internal/snapshot` | 全市场最新价快照 JSON（orchestrator fallback 查询） |

### 写入的 Redis

| 目的 | Stream/Key | 频率 | 内容 |
|---|---|---|---|
| 持久化流 | `write:tick` (Stream) | 批量 100 条/1 秒 | **全市场**合约 Tick → db_writer 落库 |
| 业务流 | `tick:<symbol>` (Stream) | 逐笔近实时 | **仅关注合约** → trading_agent 分析 |
| 合约元数据全量 | `instruments:all` (KV) | 启动时 | JSON 数组，含 ~330 合约的完整元数据（symbol/exchange/price_tick/multiplier 等） |
| 合约元数据单条 | `instrument:<symbol>` (KV) | 启动时 | 单个合约的元数据 JSON，如 `instrument:MA2509` |
| 最新价快照 | `last_price:<symbol>` (KV, JSON) | 批量 1 秒 | **全市场**合约最新价快照（TTL 60s，断线自动过期） |
| 服务注册 | `services:ctp_tick` (KV) | 10 秒续约 | ServiceInfo 30s TTL |

### 数据流（v0.3.0）

```
启动时：TD API → 查询 6 交易所 → filter ProductClass='1' → ~330 期货合约
            ├→ DeriveAllInstrumentInfos → MetadataWriter.WriteAll
            │    ├→ Redis: SET instruments:all (JSON list)
            │    ├→ Redis: SET instrument:<symbol> (each)
            │    └→ InfluxDB: POST instruments bucket (Line Protocol)
            └→ MD API.SubscribeMarketData(allSymbols)

每 Tick：CTP 回调线程（绝对不能阻塞）
            └→ tickWriter.Send() — 非阻塞 channel send (buffer 10000)
                 │
                 └→ writerLoop goroutine:
                      ├→ 攒批 100 条/1s → pipeline XADD write:tick
                      └→ if ∈ SUBSCRIBE_SYMBOLS → 逐笔 XADD tick:<symbol>
```

### 不消费任何输入
（纯生产者）

### 不暴露 gRPC
（无对外接口）

---

## 依赖

| 依赖 | 类型 | 必需 |
|---|---|---|
| Redis 7.2 | 内部服务 | ✅ |
| CTP 前置机（SimNow / 实盘） | 外部 | ✅ |
| `contracts/tick.proto` | 内部契约 | ✅ |

---

## 配置（关键环境变量）

见 `.env.example`：

```bash
SERVICE_NAME=ctp_tick
HEALTH_PORT=8081

REDIS_ADDR=redis:6379

# CTP 前置机
CTP_FRONT_ADDR=tcp://180.168.146.187:10130    # SimNow 测试
# CTP_FRONT_ADDR=tcp://实盘地址                  # 实盘切换

CTP_BROKER_ID=9999
CTP_USER_ID=<your-user-id>
CTP_PASSWORD=<your-password>
CTP_APP_ID=simnow_client_test
CTP_AUTH_CODE=0000000000000000

# 关注合约（全市场合约自动查询 + 订阅，只有这些额外推送分析层）
SUBSCRIBE_SYMBOLS=MA2509

LOG_LEVEL=info
```

---

## 启动方式

### 本地开发（mock CTP）

```bash
# 用 build tag 隔离 CTP 依赖
go build -tags="!ctp" -o ctp_tick ./cmd
./ctp_tick
```

### 生产（真 CTP）

```bash
docker compose --profile futures up -d ctp_tick
docker logs -f ctp_tick
```

---

## 测试

### Unit 测试（无外部依赖）

```bash
go test ./internal/domain/...
go test -tags="!ctp" ./internal/adapters/redis/...
```

测试 `normalizer.go` 等纯逻辑，使用 mock 替代 CTP/Redis。

### Integration 测试

```bash
# 起真 Redis + SimNow
docker compose -f docker-compose.test.yml up -d redis
go test -tags=ctp,integration ./tests/integration/...
```

---

## 已知限制 / 风险点

### 1. ⚠️ cgo 仅 Linux 可编译

CTP 库（openctp/ctp-go）依赖 Linux .so 动态库，**macOS 上无法编译完整版本**。

**解决方案**：
- 本地开发用 build tag `//go:build !ctp` 走 mock
- 生产编译走 Docker 多阶段构建（Linux 容器内）

### 2. ⚠️ SimNow 凭证申请要 1-3 工作日

去 [simnow.com.cn](http://simnow.com.cn) 注册，**等 1-3 个工作日审核**。

### 3. ⚠️ CTP 收盘强制断线

| 时段 | 事件 |
|---|---|
| 每日 15:15 | 日盘收盘断线 |
| 每日 23:00 | 夜盘收盘断线（甲醇有夜盘） |
| 节假日 | 全天不开 |

服务会自动重连（指数退避），但**重连期间 Tick 流断流**。

### 4. ⚠️ 主力合约切换

每月 15 日前后，主力合约会从近月切到远月（如 MA2509 → MA2601）。
V1 阶段需手动改 `SUBSCRIBE_SYMBOLS`，V2 计划自动检测。

### 5. ⚠️ tickChannel 满丢弃

异步管道 buffer=10000，极端行情（全市场 ~330 合约 tick 密集涌入）可能溢出。

**缓解**：
- buffer 足够容纳 330 合约 × 10 条 = 3300 条（留足余量）
- 丢弃时每 1000 条记一次 WARNING 日志
- `/metrics` 端点暴露 `redis_write_failure_total` 监控丢弃量

### 6. ⚠️ TD API 查询超时

全市场合约查询需 30 秒超时，SimNow 环境偶发超时导致回退使用 SUBSCRIBE_SYMBOLS。

**缓解**：超时后日志记录 → 降级为仅订阅关注合约 → 服务继续运行

---

## 与 ARCHITECTURE.md 的关联

- 架构图层：「数据采集层」第一个服务
- 关键技术选型：openctp/ctp-go + cgo
- 风险 1：CTP cgo 仅 Linux 可编译
- 风险 4：CTP 收盘断线 + 主力合约切换

---

## AI Agent 准则

- ✅ 改 `domain/normalizer.go` → 必须有 unit 测试
- ✅ 改 CTP 字段映射 → 同时更新 `contracts/tick.proto`
- ❌ 不要在 `domain/` 里 import `redis-go` / `openctp` / `net/http`
- ❌ 不要绕过 db_writer 直接写 InfluxDB

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版 |
| v1.1 | 2026-06-13 | 全市场合约订阅（TD API + MD API 全量）+ 异步批量落库（100条/1s）+ 关注合约近实时推送 |
