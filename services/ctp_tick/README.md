# services/ctp_tick — CTP 行情订阅服务

> 本服务属于 arbitrage-system 微服务集合的一员。
> 在动手前必读：[`/ENGINEERING_GUIDE.md`](../../ENGINEERING_GUIDE.md) + [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)

---

## 服务定位（一句话）

订阅 CTP（中国期货交易所）行情，将原始 Tick 标准化后写入 Redis Stream（让 trading_agent 消费 + 让 db_writer 持久化）。

---

## 职责

### 做什么 ✅
- 通过 cgo 封装 [`openctp/ctp-go`](https://github.com/openctp/openctp) 连接 CTP 前置机
- 订阅指定合约（V1：`MA2509` 甲醇主力；V2 加更多）
- 接收 `OnRtnDepthMarketData` 回调，标准化为 protobuf `Tick` 消息
- 写入 Redis Stream `tick:<symbol>` 供 trading_agent 消费
- 写入 Redis Stream `write:tick` 供 db_writer 持久化到 InfluxDB
- 自动重连：CTP 收盘断线（每日 15:15 / 23:00）→ 次日开盘前自动重连

### 不做什么 ❌
- 不算技术指标（那是 trading_agent.domain.indicator 的事）
- 不做交易决策
- **不直接写 InfluxDB**（统一走 db_writer 串行化，参考 ARCHITECTURE.md 机制 2）
- 不报单（那是 trading_agent.adapters.broker 的事）

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
│   │   └── normalizer.go         CTP 数据 → Tick proto 标准化
│   ├── adapters/               ← 外部 IO
│   │   ├── ctp/                  cgo 封装 openctp/ctp-go
│   │   │   ├── md.go               行情接口（Market Data）
│   │   │   ├── spi.go              CTP 回调实现
│   │   │   ├── reconnect.go        断线重连逻辑
│   │   │   └── types.go            CTP C 结构 ↔ Go struct
│   │   ├── redis/                Redis Streams 写入
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
| `cmd/main.go` | 装配（不写业务）+ 启动重连循环 |
| `domain/normalizer.go` | CTP 字段 → Tick proto 字段映射（**纯函数**） |
| `adapters/ctp/md.go` | 注册前置机、登录、订阅 |
| `adapters/ctp/spi.go` | 实现 `CThostFtdcMdSpi` 回调 |
| `adapters/ctp/reconnect.go` | 检测断线 + 指数退避重连 |
| `adapters/redis/writer.go` | XADD 到 `tick:<symbol>` 和 `write:tick` |

---

## 对外接口

### 暴露的端点

| 类型 | 端口 | 路径 | 用途 |
|---|---|---|---|
| HTTP | 8081 | `/healthz` | 健康检查（含 CTP 连接状态） |
| HTTP | 8081 | `/metrics` | Prometheus 指标 |

### 写入的 Redis

| 目的 | Stream/Key | 频率 | 内容 |
|---|---|---|---|
| 业务流 | `tick:MA2509` (Stream) | ~2 笔/秒 | proto 编码的 Tick 消息 |
| 持久化流 | `write:tick` (Stream) | 同上 | 同上（让 db_writer 写 InfluxDB） |
| 服务注册 | `services:ctp_tick` (KV) | 10 秒续约 | ServiceInfo 30s TTL |

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

# 订阅合约清单
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

### 5. ⚠️ Tick 高峰丢包风险

夜盘开盘瞬间 Tick 频率可能 ≥ 5 笔/秒。

**缓解**：
- Redis Stream 写入用 pipeline 批量
- Redis 持久化 AOF + maxmemory-policy=allkeys-lru
- 监控 Stream 长度，超过 10000 推钉钉告警

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
