# AI 辅助日内趋势交易系统 — 架构与实施方案

> 版本：v2.3（2026-06-13 更新）
> 状态：待实施
> 关联 Plan 文件：`~/.claude/plans/rippling-wibbling-feather.md`

---

## Context（项目背景）

### 这个系统要解决什么问题
个人期货交易者搭建一套**「AI 辅助 + 自动执行」的日内趋势交易系统**，主要交易**化工类期货合约（V1 锁定甲醇 MA 主力合约）**。核心痛点是个人交易者无法 7×24 盯盘，需要一套能在云端自动运行、能借助 LLM 做基本面/产业链分析、又能严格执行风控的全自动系统。

### 为什么是现在做
- 已有一份完整的《架构白皮书》构想，需要落地为可执行的工程方案
- 有完整的 Python 股票分析项目（`daily_stock_analysis/`）作为代码资产，部分模块（多平台 Bot、FastAPI 路由组织、策略模式 Fetcher）可直接迁移复用
- DeepSeek/智谱 GLM 等国产 LLM API 价格已降至个人可负担（单次评分约 ¥0.03），让"LLM 当军师"在经济上可行

### 预期成果
- 一套**全部部署在阿里云上海**的自动化交易系统
- **架构一次设计到位**（9 个微服务全部规划），**实施分 3 阶段交付**：
  - **P1 (11 周)**：核心闭环 + 基于 Vision UI 改造的高级 Dashboard，能跑 SimNow 实盘
  - **P2 (15 周)**：稳定性增强（事件驱动 AI、时段守卫、灰度发布、K 线图）
  - **P3 (18 周)**：完整版（WebUI 策略管理、舆情扩展、决策溯源页）
- 后续可平滑切换实盘 CTP 通道（仅替换前置机地址 + 账号）
- Mac 退居二线：仅做开发机和周末回测机，平时合盖不影响系统

---

## 架构原则（不可妥协的硬规则）

> 这一节是**宪法**。所有设计决策、PR review、技术争议都以这里为准。任何偏离都需要更新本节后才能进行。

### 五条硬规则

#### 规则 1：每个服务独立进程、独立 Docker 镜像、独立部署
- 一个微服务 = 一个 Docker 容器 = 一份独立的 Dockerfile
- 服务之间**绝不共享内存空间**（不允许进程内函数调用替代网络调用）
- 每个服务必须能**单独启动**调试，通过 `docker compose --profile <name> up` 实现
- 重启任意一个服务**不能**影响其他服务运行（除非是上游依赖断流，由对应服务的 graceful degradation 处理）

#### 规则 2：服务间只通过 `contracts/` 定义的协议通信
- **唯一真相源（SSOT）**：`contracts/` 顶级目录下的 `.proto` 文件
- Go/Python/未来其他语言的代码 **由 protoc 自动生成**，不允许手写 struct
- **禁止**通过 Redis 直接读写非 contract 定义的字段（如临时加个 `debug_xx` 字段绕过协议）
- 契约变更走严格流程：改 `.proto` → 跑 `make gen` → 两端代码同步 → CI 验证字段一致性

#### 规则 3：业务逻辑（domain）零外部依赖，所有 IO 在 adapters
- 每个服务内部分两层：`domain/`（纯业务）和 `adapters/`（外部 IO）
- `domain/` 里的代码：**不 import Redis、不 import InfluxDB、不 import CTP、不发 HTTP 请求**
- `domain/` 通过定义 interface（Go）或 ABC（Python）声明所需能力，由 `adapters/` 实现
- 这是 [Hexagonal Architecture / Clean Architecture] 的核心思想，让单元测试可以**毫秒级跑完且脱网**

#### 规则 4：每个服务必须有 `/healthz` 和 `/metrics` 端点
- `/healthz`：返回服务是否健康（依赖的下游是否在线、是否能正常处理请求）
- `/metrics`：Prometheus 格式的监控指标（QPS、延迟、错误数、业务指标）
- Watchdog 容器周期性巡检所有服务的 `/healthz`，任意一个不健康即推钉钉
- 即使 V1 没有 Prometheus 服务器，`/metrics` 端点也要先实现（成本极低，未来零迁移）

#### 规则 5：单元测试不能依赖外部服务
- `tests/unit/` 下的测试必须能在**没有 Redis、没有 InfluxDB、没有网络**的环境下跑通
- 外部依赖一律用 mock / fake 替代（`adapters/` 层的 interface 让这件事变得简单）
- `tests/integration/` 才能用真 Redis/InfluxDB（通过 docker-compose.test.yml 启动）
- CI 中：unit 测试**每个 PR 必跑**，integration 测试只在 nightly 跑

### 仓库形态：Monorepo

**一个 git 仓库（`arbitrage-system`）下放多个独立模块**，不走 polyrepo（多仓库）路线。

| 决策维度 | 结论 |
|---|---|
| 仓库数量 | 1 个 |
| 跨模块改动（如改契约）| 1 个 PR 改完所有模块 |
| 单元测试 | 每个模块独立跑，可并行 |
| Docker 镜像 | 每个模块独立打镜像，独立标签 |
| 部署 | 通过 docker-compose 统一编排，但每个模块可独立重启 |

**为什么 monorepo 不是反微服务**：微服务的核心是**运行时解耦**（独立进程、独立部署），不是**代码仓库隔离**。Google/Facebook 都是 monorepo + 微服务。

### 服务间通信协议分层

不再笼统说"走 Redis"，按用途分清：

| 通信场景 | 协议 | 实现 |
|---|---|---|
| **事件流**（Tick 推送、AI 评分更新、信号广播） | Redis Streams（不是 Pub/Sub） | 持久化、有 offset、可重放 |
| **命令调用**（手动平仓、查询持仓、修改参数） | HTTP / gRPC | 同步、有响应、便于追溯 |
| **共享状态**（最新评分、风控状态） | Redis KV + TTL | 读多写少、需要老化机制 |
| **持久化**（Tick 历史、订单流水） | InfluxDB / SQLite | 时序数据 / 关系数据分流 |
| **健康巡检** | HTTP `/healthz` | Watchdog 调用，标准化 |

### Broker 抽象层（为未来股票接入预留）

在 `go-engine/internal/broker/` 中定义 **Broker interface**，CTP 是其首个实现。未来加股票（QMT / easytrader / 其他）时，只需新增一个 `adapters/<broker>_impl.go`，**业务逻辑层零修改**。

```go
// 概念伪代码
type Broker interface {
    Subscribe(symbols []string) error
    PlaceOrder(req OrderRequest) (OrderID, error)
    CancelOrder(id OrderID) error
    QueryPositions() ([]Position, error)
    OnTick(handler func(Tick))
}
```

V1 只实现 `CTPBroker`；V2 加股票时实现 `QMTBroker` 或 `EasyTraderBroker`。Strategy 层通过 interface 调用，永远不直接依赖任何具体 broker。

---

## 服务拓扑与详细架构图

> 这是 v1.5 的**核心升级**：明确了**两层 agent 架构**——`orchestrator-agent`（编排层）+ `trading-agent`（决策层）+ 数据采集服务族。

### 详细架构图

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                  ☁️ 阿里云上海 ECS 4C8G u2a · Ubuntu 22.04 · Docker Compose         │
├──────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌──────────────────────────────────────────────────────────────────────────┐    │
│  │ 📡 数据采集层 (Data Ingestion Layer) — 被动产出，无业务决策                 │    │
│  │ ────────────────────────────────────────────────────────────────────     │    │
│  │                                                                          │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                    │    │
│  │  │ 📈 ctp_tick   │  │ 📊 stock_tick │  │ 🤖 llm_scorer │                   │    │
│  │  │   (Go)       │  │   (Go, V2)   │  │  (Python)    │                    │    │
│  │  │              │  │              │  │              │                    │    │
│  │  │ • 订阅 CTP   │  │ • 订阅股票   │  │ • DeepSeek   │                    │    │
│  │  │ • Tick 标准化│  │   行情       │  │ • GLM-4 兜底  │                   │    │
│  │  │ • 写 Redis   │  │ • 写 Redis   │  │ • 5min 评分  │                    │    │
│  │  │   Streams    │  │   Streams    │  │ • 写 Redis KV│                    │    │
│  │  │ Port: -      │  │ Port: -      │  │ Port: 5000   │                    │    │
│  │  │ /healthz     │  │ /healthz     │  │ /healthz     │                    │    │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                    │    │
│  │         │                 │                  │                           │    │
│  │         └────────┬────────┘                  │                           │    │
│  │                  ▼                           │                           │    │
│  │           ┌──────────────┐                   │                           │    │
│  │           │ 📐 indicator  │                   │                           │    │
│  │           │   (Go)       │                   │                           │    │
│  │           │              │                   │                           │    │
│  │           │ • MA/Boll/ATR│                   │                           │    │
│  │           │ • 量比/动量  │                   │                           │    │
│  │           │ • 写 Redis KV│                   │                           │    │
│  │           │ Port: -      │                   │                           │    │
│  │           │ /healthz     │                   │                           │    │
│  │           └──────┬───────┘                   │                           │    │
│  └──────────────────┼───────────────────────────┼───────────────────────────┘    │
│                     │                           │                                │
│                     └─────────────┬─────────────┘                                │
│                                   ▼                                              │
│  ┌──────────────────────────────────────────────────────────────────────────┐    │
│  │ 🚌 消息总线层 (Message Bus Layer)                                          │    │
│  │ ────────────────────────────────────────────────────────────────────     │    │
│  │                                                                          │    │
│  │   ┌───────────────────────────────────────────────────────────────┐      │    │
│  │   │ 🔴 Redis 7.2                                                    │      │    │
│  │   │ ─────────────────────────────────────────                      │      │    │
│  │   │ Streams:                                                        │      │    │
│  │   │   • tick:MA2509                — Tick 流（保留 1h）              │      │    │
│  │   │   • signal:MA2509              — 策略信号流                      │      │    │
│  │   │   • order_event                — 报单事件流                      │      │    │
│  │   │ KV:                                                             │      │    │
│  │   │   • ai_score:MA2509:live       — 实际生效评分                    │      │    │
│  │   │   • ai_score:MA2509:shadow     — 影子评分（仅记录）               │      │    │
│  │   │   • indicator:MA2509:current   — 最新指标值                      │      │    │
│  │   │   • engine_status              — trading-agent 运行状态         │      │    │
│  │   │   • position:current           — 当前持仓                        │      │    │
│  │   │ Port: 6379（仅内网）                                             │      │    │
│  │   └───────────────────────────────────────────────────────────────┘      │    │
│  └──────────────────────────────────────────────────────────────────────────┘    │
│                                   ▲                                              │
│                                   │ 订阅事件 / 读写状态                            │
│                                   │                                              │
│  ┌────────────────────────────────┼─────────────────────────────────────────┐    │
│  │ 🧠 决策层 (Decision Layer)      │                                         │    │
│  │ ────────────────────────────────────────────────────────────────────     │    │
│  │                                 │                                         │    │
│  │   ┌─────────────────────────────┴────────────────────────────────┐      │    │
│  │   │ 🟢 trading-agent (Go) ⭐ 决策大脑                                │      │    │
│  │   │ ─────────────────────────────────                              │      │    │
│  │   │ 职责：                                                          │      │    │
│  │   │   • 订阅 Redis（tick + indicator + ai_score）                   │      │    │
│  │   │   • 跑策略融合（domain.strategy）                               │      │    │
│  │   │   • AI 评分老化（< 15min/15-30min/>30min decay 1.0/0.5/0.0）    │      │    │
│  │   │   • 决策融合（finalBias = score×conf×decay×alpha）              │      │    │
│  │   │   • 风控闸门（domain.risk）                                     │      │    │
│  │   │   • Broker interface 报单（V1: CTP）                             │      │    │
│  │   │   • 持仓管理 + 紧急停机                                          │      │    │
│  │   │ 暴露：                                                          │      │    │
│  │   │   • gRPC :9090（仅供 orchestrator 调用，不暴露公网）              │      │    │
│  │   │   • HTTP :8081/healthz, :8081/metrics                            │      │    │
│  │   │ 状态：                                                          │      │    │
│  │   │   • 全部存 Redis（重启从 Redis 恢复）                            │      │    │
│  │   │   • Docker restart: always                                      │      │    │
│  │   └────────┬─────────────────────────────────────────────────────┘      │    │
│  │            │                                                            │    │
│  └────────────┼────────────────────────────────────────────────────────────┘    │
│               │ gRPC（Tailscale 内网）                                          │
│               │                                                                │
│  ┌────────────┼────────────────────────────────────────────────────────────┐    │
│  │ 🌐 编排层 (Orchestration Layer)                                         │    │
│  │ ────────────────────────────────────────────────────────────────────     │    │
│  │            │                                                             │    │
│  │   ┌────────▼─────────────────────────────────────────────────────┐      │    │
│  │   │ 🟦 orchestrator-agent (Go) ⭐ 路由 + 协调中枢                     │      │    │
│  │   │ ─────────────────────────────────                              │      │    │
│  │   │ 职责：                                                          │      │    │
│  │   │   • REST API + WebSocket（对 dashboard-web）                    │      │    │
│  │   │   • 聚合 Redis 状态 + trading-agent gRPC 状态                   │      │    │
│  │   │   • 转发 WebUI 指令到 trading-agent                              │      │    │
│  │   │     - 一键平仓                                                  │      │    │
│  │   │     - 暂停/恢复策略                                              │      │    │
│  │   │     - 修改风控参数                                               │      │    │
│  │   │   • 操作日志（审计追溯）                                          │      │    │
│  │   │   • 认证 + 限流（V2 加）                                         │      │    │
│  │   │   • 跨模块事务编排（如批量平仓）                                  │      │    │
│  │   │ 暴露：                                                          │      │    │
│  │   │   • HTTP :8080/api/v1/*（REST）                                 │      │    │
│  │   │   • WS :8080/ws（实时推送）                                     │      │    │
│  │   │   • HTTP :8080/healthz, :8080/metrics                           │      │    │
│  │   │ 不做：                                                          │      │    │
│  │   │   • 不做交易决策（那是 trading-agent 的事）                      │      │    │
│  │   │   • 不直接操作 Broker                                            │      │    │
│  │   │   • 不持久化业务状态（无状态）                                    │      │    │
│  │   └────────────┬─────────────────────────────────────────────────┘      │    │
│  │                │ HTTP / WebSocket                                       │    │
│  └────────────────┼───────────────────────────────────────────────────────┘    │
│                   │                                                            │
│  ┌────────────────┼───────────────────────────────────────────────────────┐    │
│  │ 🎨 表现层 + 监控层                                                       │    │
│  │ ────────────────────────────────────────────────────────────────────     │    │
│  │                │                                                         │    │
│  │   ┌────────────▼─────────────┐   ┌────────────────────────────┐         │    │
│  │   │ 🟨 dashboard-web (Go)    │   │ 🟧 watchdog (Go)            │         │    │
│  │   │ ──────────────────────   │   │ ──────────────────────────  │         │    │
│  │   │ • 静态 HTML + HTMX       │   │ • 巡检各服务 /healthz        │         │    │
│  │   │ • 模板渲染                │   │ • 监控磁盘使用率              │         │    │
│  │   │ • 不连 Redis              │   │ • 监控 LLM 评分新鲜度         │         │    │
│  │   │ • 不连 trading-agent     │   │ • 监控 CTP 连接状态           │         │    │
│  │   │ • 仅调 orchestrator API  │   │ • 推钉钉 Webhook              │         │    │
│  │   │ Port: 8000               │   │ Port: -                      │         │    │
│  │   └──────────────────────────┘   └────────┬─────────────────────┘         │    │
│  └─────────────────────────────────────────┼───────────────────────────────┘    │
│                                            │ HTTPS                              │
│                                            ▼                                    │
│                                     ┌──────────────┐                            │
│                                     │ 钉钉群机器人  │                             │
│                                     └──────────────┘                            │
│                                                                                  │
│  ┌──────────────────────────────────────────────────────────────────────────┐    │
│  │ 💾 持久化层 (Persistence Layer)                                            │    │
│  │ ────────────────────────────────────────────────────────────────────     │    │
│  │                                                                          │    │
│  │   ┌─────────────────────────────────────────────────────────────┐        │    │
│  │   │ 🟪 InfluxDB 2.7                                                │        │    │
│  │   │ ─────────────────────────────────────────                     │        │    │
│  │   │ Bucket: ma_tick           — Tick 全量（30 天）                  │        │    │
│  │   │ Bucket: ma_1min           — 1 分钟 K 线（90 天）                │        │    │
│  │   │ Bucket: indicator         — 技术指标（30 天）                    │        │    │
│  │   │ Bucket: ai_score_history  — AI 评分历史（30 天）                 │        │    │
│  │   │ Bucket: order_history     — 报单流水（永久）                     │        │    │
│  │   │ Port: 8086（仅内网）                                            │        │    │
│  │   │                                                                │        │    │
│  │   │ 谁写 InfluxDB：                                                 │        │    │
│  │   │   • ctp_tick 写 ma_tick                                         │        │    │
│  │   │   • indicator 写 indicator + ma_1min                            │        │    │
│  │   │   • llm_scorer 写 ai_score_history                              │        │    │
│  │   │   • trading-agent 写 order_history                              │        │    │
│  │   └─────────────────────────────────────────────────────────────┘        │    │
│  └──────────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
                                  ↑                       ↓
                                  │ Tailscale (100.x.x.0/10)  │ CTP 协议（公网）
                                  │                           │
                          ┌───────┴────────┐         ┌───────▼─────────┐
                          │ 💻 你的设备     │         │ SimNow 模拟柜台  │
                          │ • Mac          │         │ → 实盘 CTP（V2）│
                          │ • 手机         │         └─────────────────┘
                          │ 浏览器访问     │
                          │ http://ali-trade:8000 │
                          └────────────────┘
```

### 服务清单（v1.5 最终版）

| # | 服务 | 语言 | 镜像大小 | 内存 | 职责 |
|---|---|---|---|---|---|
| 1 | **ctp_tick** | Go | ~30MB | 0.2G | 订阅 CTP 行情 |
| 2 | **stock_tick** (V2) | Go | ~30MB | 0.2G | 订阅股票行情（V1 不做） |
| 3 | **indicator** | Go | ~20MB | 0.2G | 计算技术指标 |
| 4 | **llm_scorer** | Python | ~200MB | 0.8G | LLM 评分 |
| 5 | **trading-agent** ⭐ | Go | ~30MB | 0.4G | 决策大脑、风控、报单 |
| 6 | **orchestrator-agent** ⭐ | Go | ~25MB | 0.3G | WebUI 路由 + 状态聚合 |
| 7 | **dashboard-web** | Go | ~20MB | 0.2G | 静态展示 |
| 8 | **watchdog** | Go | ~20MB | 0.1G | 健康监控 + 告警 |
| 9 | **redis** | - | ~30MB | 0.5G | 消息总线 |
| 10 | **influxdb** | - | ~120MB | 2.0G | 历史存储 |
| | **小计** | | **~525MB** | **5.7G** | |
| | **8G 预算余量** | | | **2.3G** | 突发缓冲 |

✅ **4C8G 服务器仍然够用**（不需要升配置）。

### 两层 agent 的关键设计哲学

#### 上层 `orchestrator-agent`：「面向人类的代理」
- 关心：UI 协议、状态聚合、指令路由、操作审计
- **不做交易决策**（这是 trading-agent 的事）
- **无业务状态**（重启不丢任何数据，因为只是路由层）
- 未来加用户登录、操作权限、操作审计都在这里

#### 下层 `trading-agent`：「面向机器的代理」
- 关心：策略、风控、报单、持仓
- **不直接面对人类**（所有人类指令通过 orchestrator 转发）
- **状态全部存 Redis**（重启从 Redis 恢复）
- 未来加股票时此层可裂变（trading-agent-futures + trading-agent-stocks），但 orchestrator 不变

#### 为什么这样分层

1. **职责完全分离**：交易策略和 UI 协议是两件本质不同的工程问题
2. **dashboard 不被业务污染**：dashboard 只调 `orchestrator.GET /api/v1/overview`，不需要懂 Redis key 命名
3. **未来扩展便宜**：加股票时只需要扩 trading-agent，orchestrator 加几个路由即可
4. **故障隔离**：orchestrator 挂了，trading-agent 仍然在跑（交易不停，只是 Web UI 不能看）；trading-agent 挂了，watchdog 立即告警 + Docker 自动重启

### 核心通信协议

| 通信对 | 协议 | 端口 | 用途 |
|---|---|---|---|
| ctp_tick → Redis | Redis Streams | 6379 | Tick 推送 |
| llm_scorer → Redis | Redis KV + TTL | 6379 | AI 评分 |
| indicator → Redis | Redis Streams + KV | 6379 | 指标更新 |
| trading-agent ↔ Redis | Redis Streams + KV | 6379 | 订阅事件 + 读写状态 |
| trading-agent → CTP | CTP 二进制协议 | 公网 | 报单 |
| **orchestrator → trading-agent** | **gRPC** | **9090** | **指令调用 + 状态查询** |
| dashboard-web → orchestrator | HTTP REST + WebSocket | 8080 | 用户操作 |
| watchdog → 各服务 | HTTP /healthz | 各服务 | 健康巡检 |
| watchdog → 钉钉 | HTTPS Webhook | 公网 | 告警推送 |
| 各服务写 InfluxDB | InfluxDB Line Protocol | 8086 | 历史持久化 |

### 服务清单（v2.2 修订版：9 个微服务）

> v2.2 变更：因 Dashboard 升级为 Next.js + TypeScript（异构语言），从 orchestrator 重新拆出独立服务。
> 8 → 9 个服务（dashboard_web 重新独立）。

#### 合并明细
| 之前（v2.0） | 现在（v2.2） | 合并理由 |
|---|---|---|
| `indicator`（独立） | 合并入 `trading_agent` 的 `domain/indicator/` 包 | 生命周期完全绑定 + 减少 Redis 跳数 |
| ~~v2.1: dashboard_web 合入 orchestrator~~ | **v2.2 重新拆出**（Next.js + TypeScript） | 异构语言无法合并 |
| `domestic_intel` + `overseas_intel` + `social_intel` | 合并为单一 `intel_collector` | 都是 Python + 共享清洗代码 + 配置驱动 |

#### 最终服务清单

| # | 服务 | 类型 | P1 内存 | 角色 |
|---|---|---|---|---|
| 1 | **redis** | 基础设施 | 1.0 GB | 消息总线 + 共享状态 |
| 2 | **influxdb** | 基础设施 | 2.0 GB | 历史数据 + 审计 |
| 3 | **ctp_tick** | 数据采集 | 0.2 GB | CTP 行情订阅（cgo 必须独立） |
| 4 | **intel_collector** ⭐ | 数据采集 | 0.6 GB | 三层舆情统一采集（配置驱动） |
| 5 | **llm_scorer** | 数据加工 | 0.8 GB | LLM 评分（Python 必须独立） |
| 6 | **trading_agent** ⭐ | 决策核心 | 0.6 GB | 含 indicator/strategy/decision/risk |
| 7 | **orchestrator** | 编排 | 0.4 GB | gRPC 客户端 + REST API + WebSocket（不含 UI） |
| 8 | **dashboard_web** ⭐v2.2 | 表现层 | 0.1 GB | Next.js 静态导出 + nginx 托管 |
| 9 | **db_writer** | 持久化 | 0.4 GB | InfluxDB 串行写入 |
| 10 | **watchdog** | 监控 | 0.1 GB | 健康巡检 + 钉钉 |
| **小计**（不含系统） | | **6.2 GB** | |
| 系统 + Docker | - | 1.3 GB | |
| **总计** | | **7.5 GB** | ✅ 4C8G 充裕 |

> 备注：`stock_tick` (V2 占位) 不计入。

#### dashboard_web 资源占用极轻

由于用 nginx 托管静态文件（不需要 Node 运行时），dashboard_web 容器：
- 镜像大小：~25 MB（nginx:alpine + 静态资源）
- 内存：~50 MB
- CPU：~5%（仅静态托管）

✅ 重新拆出独立服务**几乎零资源代价**。

#### 真正的收益

| 维度 | 12 服务（v2.0） | 9 服务（v2.2） | 改进 |
|---|---|---|---|
| **容器数** | 12 个 | 9 个 | **-25%** |
| **配置项** | ~120 个 | ~90 个 | **-25%** |
| **启动时间** | ~30 秒 | ~18 秒 | **-40%** |
| **重复代码（Python 爬虫）** | 3 套骨架 | 1 套 | **-66%** |
| **Docker 镜像数** | 12 个 build | 9 个 build | **-25%** |

---

## 关键机制设计（6 个核心机制）

### 机制 1：策略与指标的 WebUI 管理 + 交易时段守卫

> 「策略肯定是通过 WebUI 输入的，但要加限制：交易时段新增的策略和指标不能生效，非交易时段才能新增。」

#### 表达形式：YAML 配置

策略和指标都用 YAML 文件描述，存放在 InfluxDB（带版本）+ 在线 hot-load 到内存。延续老项目（`daily_stock_analysis/strategies/*.yaml`）的成功模式。

```yaml
# 策略示例
name: ma_cross_v1
type: strategy
symbol: MA2509
description: 双均线趋势跟随
params:
  fast_ma: 5
  slow_ma: 20
  ai_alpha: 0.2          # AI 评分参与权重
  position_size: 1
  stop_loss_pct: 0.005
mode: shadow             # shadow（影子模式）/ live（实盘）
created_at: 2026-06-15T20:00:00+08:00
created_by: jacklee
activated_at: null       # 提升到 live 的时间，null 表示还没激活
```

```yaml
# 指标示例
name: bollinger_band_20
type: indicator
symbol: MA2509
formula: bollinger
params:
  period: 20
  std_dev: 2.0
output_to: indicator:MA2509:boll_20
```

#### 交易时段守卫（Trading Session Guard）

**所有"修改型操作"在交易时段被拒绝**：

| 操作 | 交易时段 | 非交易时段 |
|---|---|---|
| 新增/编辑/删除策略 | ❌ 拒绝 | ✅ 允许 |
| 新增/编辑/删除指标 | ❌ 拒绝 | ✅ 允许 |
| 修改风控参数 | ❌ 拒绝 | ✅ 允许 |
| 启动/停止策略（live ↔ shadow 切换） | ❌ 拒绝 | ✅ 允许 |
| **紧急平仓**（特殊例外） | ✅ **允许**（关乎安全） | ✅ 允许 |
| 查询/查看 | ✅ 允许 | ✅ 允许 |

**实现位置**：`orchestrator-agent` REST API 中间件层

```go
// services/orchestrator_agent/internal/api/rest/middleware.go
func TradingSessionGuard(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if isStrategyMutation(r) && isInTradingSession(time.Now()) {
            http.Error(w,
                fmt.Sprintf("禁止在交易时段修改策略。下次可操作时间：%s",
                    nextNonTradingWindow().Format("15:04")),
                http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

#### 灰度发布流程

新策略不能立即全仓应用：

```
非交易时段：用户上传新策略 (mode=shadow)
       ↓
       策略写入 Redis + InfluxDB（持久化）
       ↓
开盘后：trading-agent 加载所有 mode=shadow + mode=live 策略
       ↓
       shadow 策略：只生成信号，不下单，写 InfluxDB 记录
       live 策略：正常生成信号 + 下单
       ↓
N 个交易日后（用户决定）：
       用户在 Dashboard 手动「提升为 live」（这个操作也只能在非交易时段做）
       ↓
       策略 mode 改为 live，下次开盘生效
```

#### 中国期货交易日历

```go
// services/orchestrator_agent/internal/domain/calendar/futures.go
var futuresSessions = []TradingSession{
    {Start: "09:00", End: "10:15"},   // 上午前盘
    {Start: "10:30", End: "11:30"},   // 上午后盘
    {Start: "13:30", End: "15:00"},   // 下午盘
    {Start: "21:00", End: "23:00"},   // 夜盘（甲醇）
}

// 节假日清单（每年初手动更新；后期可对接交易所 API）
var marketHolidays2026 = []string{
    "2026-01-01", "2026-02-15", ... // 元旦、春节、清明、劳动节、端午、中秋、国庆
}
```

---

### 机制 2：DB 写入串行化（db_writer 服务）

> 「写 db 的只能有一个节点写，其他节点只把数据交给他，他进行异步的、按照队列依次写 db。」

#### 设计：CQRS 写侧 + Redis Stream 队列

```
 ctp_tick      indicator    llm_scorer    trading_agent    sentiment_monitor
    │             │              │              │                │
    │ XADD        │ XADD         │ XADD         │ XADD           │ XADD
    ▼             ▼              ▼              ▼                ▼
   ┌─────────────────────────────────────────────────────────────┐
   │ Redis Streams（独立写入流，不与业务流混淆）                     │
   │   • write:tick                                                │
   │   • write:indicator                                           │
   │   • write:score                                               │
   │   • write:order                                               │
   │   • write:sentiment                                           │
   │   • write:position_snapshot                                   │
   └─────────────────────────────┬──────────────────────────────┘
                                 │ XREADGROUP
                                 ▼
                    ┌─────────────────────────┐
                    │ 🟪 db_writer (Go)        │
                    │ ──────────────────────  │
                    │ • 单消费者实例（防并发）  │
                    │ • 多消费组（每流一组）    │
                    │ • 批量聚合（100 条 / 1s）│
                    │ • InfluxDB 批量写入      │
                    │ • XACK 成功后确认        │
                    │ • 失败 → 死信队列        │
                    │ • 写入指标暴露 /metrics  │
                    └────────────┬─────────────┘
                                 │
                                 ▼
                          ┌──────────────┐
                          │ InfluxDB 2.7 │
                          └──────────────┘
```

#### 关键特性

| 特性 | 说明 |
|---|---|
| **单实例消费** | 同一时间只有一个 db_writer 进程在跑（避免并发写竞争） |
| **批量写入** | 默认 100 条 / 1 秒（取先满足者）触发 batch flush |
| **顺序保证** | Redis Stream 天然有序，db_writer 按入流顺序 ACK |
| **失败重试** | 写 InfluxDB 失败 → 重试 3 次（指数退避） → 失败转死信队列 `dlq:write:*` |
| **死信告警** | 死信队列有数据 → 推钉钉 + Dashboard 红灯 |
| **业务零阻塞** | 业务服务只 XADD（毫秒级），不等 InfluxDB 响应 |

#### 性能预估

- 单合约甲醇 Tick：~2 笔/秒 → 批量 100 条聚合后 50 秒一批
- 全部数据流合并：约 5-10 笔/秒入流
- InfluxDB 批量写入吞吐：~10000 条/秒
- ✅ **db_writer 单实例完全够用**，远不到瓶颈

---

### 机制 3：数据可靠性（Redis 持久化 + 启动对账）

> 「Redis 核心数据要做持久化，或者启动的时候从 db 加载近期的数据，不能完全丢失。」

#### 三层防护

##### 第一层：Redis 自身持久化

```yaml
# docker-compose.yml 中的 Redis 配置
redis:
  image: redis:7.2-alpine
  command: >
    redis-server
    --appendonly yes
    --appendfsync everysec
    --save 900 1
    --save 300 10
    --save 60 10000
    --maxmemory 1gb
    --maxmemory-policy allkeys-lru
  volumes:
    - ./data/redis:/data       # 持久化到 ECS 磁盘
```

效果：每秒 fsync 到磁盘，Redis 容器重启不丢任何数据。

##### 第二层：关键数据双写

| 数据 | 写 Redis | 双写 InfluxDB | 终极兜底 |
|---|---|---|---|
| 当前持仓 | ✅ KV | ✅ position_snapshot 流 | **CTP 查询接口** |
| AI 评分（live + shadow） | ✅ KV | ✅ ai_score_history | InfluxDB 最近 1h |
| 策略参数 | ✅ KV | ✅ strategy_versions | InfluxDB 最新版本 |
| 指标参数 | ✅ KV | ✅ indicator_versions | InfluxDB 最新版本 |
| 报单流水 | ✅ Stream | ✅ order_history（永久） | InfluxDB 永久 |
| Tick 数据 | ✅ Stream（短期保留） | ✅ ma_tick（30 天） | InfluxDB |

##### 第三层：启动时三步对账（trading-agent 启动流程）

```go
// services/trading_agent/cmd/main.go
func main() {
    cfg := config.Load()
    rdb := connectWithRetry("redis:6379", 60*time.Second)
    influx := connectWithRetry("http://influxdb:8086", 60*time.Second)

    // ⭐ 三步对账
    state := State{}

    // Step 1: 从 Redis 读最新状态
    state.Position = rdb.Get("position:current")
    state.LiveScore = rdb.Get("ai_score:MA2509:live")
    state.Strategies = rdb.Get("strategies:active")
    log.Info("Step 1: Redis 状态加载完成")

    // Step 2: 从 InfluxDB 补齐最近 1 小时数据
    if state.Position == nil {
        state.Position = influx.QueryLatestPositionSnapshot()
        log.Warn("Step 2: Redis 无持仓，从 InfluxDB 恢复")
    }

    // Step 3: ⭐⭐ CTP 查询账户真实持仓 → 与本地状态对账
    ctpPosition := ctp.QueryAccountPosition(timeout=30s)
    if !state.Position.Equals(ctpPosition) {
        // 不一致：写 InfluxDB 报警事件 + 推钉钉
        alertEvent := AlertEvent{
            Type:     "position_mismatch",
            Severity: "critical",
            Local:    state.Position,
            Remote:   ctpPosition,
            Message:  "本地持仓与 CTP 不一致，启动 SafeMode",
        }
        publishAlert(alertEvent)

        // 切"SafeMode"模式（只平仓不开新仓）
        state.Mode = ModeSafe
        log.Error("Step 3: 持仓对账失败，进入 SafeMode")
    } else {
        state.Mode = ModeNormal
        log.Info("Step 3: 持仓对账成功")
    }

    runEngine(state)
}
```

**核心原则**：**CTP 查询是黄金真相**，本地任何状态都是缓存。

##### 各服务的启动恢复策略

| 服务 | 启动恢复内容 |
|---|---|
| ctp_tick | 无状态，直接订阅即可 |
| indicator | 启动时不做计算，等 Tick 来再算（自然恢复） |
| llm_scorer | 启动时检查 Redis 是否有最近评分；无则立即触发一次 |
| trading-agent | ⭐ 三步对账（如上） |
| orchestrator | 无业务状态，直接启动 |
| dashboard-web | 无状态 |
| watchdog | 无状态 |
| db_writer | 从死信队列读未处理项 → 优先处理 |
| sentiment_monitor | 从 InfluxDB 读最近抓取时间 → 从那里继续 |

---

### 机制 4：事件驱动 AI 介入（紧急触发）

> 「当决策层发现指标、数据出现单向剧烈波动的时候，要立刻询问 LLM 层是否有新的突发事件。」

#### 触发条件（任一即触发）

trading-agent 的 `onTick` 回调中持续监控：

| 触发指标 | 阈值 | 计算方式 |
|---|---|---|
| 1 分钟价格涨跌幅 | > ±1.0% | 当前价 vs 1 分钟前价 |
| 量比 | > 3.0 | 最近 1 分钟成交量 / 过去 N 分钟均值 |
| 单向成交比 | > 0.85 | 主动买/卖单占比 |
| 价格突破布林上/下轨 | 任一 | 比照 boll_20 |
| 持仓盈亏剧变 | 单分钟 ±2% | 浮动盈亏波动 |

#### 防抖

5 分钟内最多触发一次紧急分析，避免 LLM 连续被打满。

#### 紧急分析流程

```
trading-agent 检测到剧烈波动
   │ POST /emergency_analysis
   ▼
llm_scorer 收到紧急请求
   │
   ├─ 立即抓最近 30 分钟相关新闻（财联社/卓创/雪球甲醇话题）
   ├─ 从 sentiment_monitor 拉最近舆情快照
   ├─ 用专用 EMERGENCY_PROMPT 调 LLM（timeout 30s，比常规 60s 短）
   │
   ▼
生成"紧急评分" → 写 Redis: ai_score:MA2509:emergency
   │ TTL 10 分钟
   │
   ▼
trading-agent 检测到 emergency 评分 → 切"谨慎模式"
   │
   ├─ 暂停开新仓 5 分钟（默认）
   ├─ 加紧现有持仓止损 50%（默认）
   ├─ 用户配置可选：仓位减半 / 一键平仓
   │
   ▼
推钉钉告警："🚨 行情异动 - {symbol}\n{LLM 分析摘要}"
```

#### 紧急评分数据结构

```protobuf
// contracts/ai_score.proto 扩展
message EmergencyAIScore {
  AIScore base = 1;              // 基础评分结构
  
  string trigger_reason = 2;     // "price_spike" / "volume_surge" / "boll_breakout"
  
  message Volatility {
    double price_change_pct_1m = 1;
    double volume_multiplier = 2;
    double one_sided_ratio = 3;
  }
  Volatility volatility = 3;
  
  repeated string breaking_news = 4;   // 抓到的紧急新闻标题
  
  string suggested_action = 5;   // "pause_new_orders" / "tighten_stops" / "halve_position" / "emergency_close"
  double action_confidence = 6;
}
```

---

### 机制 5：舆情监控（三层数据源体系）

> v1.9 升级：原 `sentiment_monitor` 单服务拆分为三层数据源体系，按合规性 + 实施顺序分服务。

#### 服务拆分

舆情监控不是一个服务，而是**三个独立的微服务**（按数据源出处分层）：

| 服务 | 数据源类型 | 出海需求 | 实施阶段 |
|---|---|---|---|
| `domestic_intel` | 国内 RSS / 论坛 / 资讯网站 | ❌ 不需要 | **P1 (M4)** + 扩展 P2 (M8) |
| `overseas_intel` | 海外 RSS / 公司公告 / 行业月报 | ✅ 需要香港跳板 | **P3 (M11)** |
| `social_intel` | Telegram 群组监听 | ✅ 需要香港跳板 | **V4 (M12+)** 后续 |

#### Layer 1: domestic_intel（国内源）

**P1 V1 范围（M4 第 5-6 周，1 周工作量）**：
- 财联社化工频道 RSS（**单源单频**：5 分钟一次）
- 简单 LLM 情感打分（-1/0/+1 三档）
- 写 Redis Stream `news_stream` + 通过 db_writer 写 InfluxDB

**P2 V2 扩展（M8 第 11-12 周）**：
- 增加：东方财富股吧（甲醇吧）
- 增加：雪球甲醇话题
- 增加：卓创资讯（如订阅）/ 隆众资讯
- ⭐ **RSS 动态新增 UI**（Dashboard 上可增删查改 RSS 源）

#### Layer 2: overseas_intel（海外源）

**P3 V3 范围（M11 第 16 周，2 周工作量含香港跳板搭建）**：
- OilPrice.com（原油新闻 RSS）
- Reuters Energy RSS
- Methanex（甲醇龙头公司公告）
- IEA（国际能源署月报）

**所有海外请求必须走香港跳板**（见下面"出海方案"）。

#### Layer 3: social_intel（社交源）

**V4 后续阶段（M12+，2 周工作量）**：
- Telegram 群组监听（化工/能源公开群）
- 使用 `python-telegram-bot` 或 `telethon`
- 走香港跳板 + Telegram 国际节点

**明确不做**：
- ❌ Discord 监听（中文化工圈使用率 < 5%，性价比极低）
- ❌ X (Twitter) 监听（API $200/月起步，第三方爬虫不稳定，X 反爬严格）
- ❌ 微博热搜（企业 API 资质难拿）

#### 出海方案：阿里云香港跳板

**为什么不能用商业代理 / 自建 VPS**：在中国大陆的法律灰色地带，企业 / 个人合规风险高。

**采用合规方案**：阿里云香港节点 ECS 1C1G（约 ¥80-150/月，**仅在 P3 启用海外源时购买**）

```
┌──────────────────────────────────────────┐
│ 上海 ECS（4C8G u2a）                       │
│  ┌───────────────────────┐               │
│  │ overseas_intel 容器    │ HTTP_PROXY   │
│  │ social_intel 容器     │ SOCKS5       │
│  └────────────┬──────────┘               │
│               │ Tailscale 内网或专线      │
└───────────────┼──────────────────────────┘
                │
┌───────────────┼──────────────────────────┐
│ 香港 ECS（1C1G）                           │
│  ┌────────────▼──────────┐               │
│  │ v2ray / squid 代理     │ → 公网         │
│  └───────────────────────┘               │
└──────────────────────────────────────────┘
                │
                ▼
       Telegram / Reuters / OilPrice / Methanex
```

#### 数据流（统一）

```
N 个 intel 服务（domestic / overseas / social）
   │
   ├─ 多线程抓取数据源（按各源独立频率）
   ├─ 文本归一化（去 HTML、统一编码）
   ├─ 关键词过滤（"甲醇/MA/原油/煤炭/港口/产能"等）
   ├─ 跨源去重（基于内容 SimHash）
   │
   ▼
XADD news_stream
   │
   ├─→ XADD write:sentiment（db_writer 写 InfluxDB 永久保存）
   └─→ llm_scorer 订阅
            │
            ├─ 每条调 LLM 打"情感分"（-1 ~ +1）
            ├─ 提取关键事件类型
            ├─ 合成"舆情指数"
            │
            ▼
        SET sentiment:MA2509:current（TTL 1 小时）
```

#### 共享基础设施（contracts + 库）

```
contracts/
├── intel_event.proto         # 统一情报事件格式
├── rss_source.proto          # RSS 源配置（动态管理用）

shared libs（pkg/intel/）
├── normalizer.py             # 文本归一化
├── deduper.py                # SimHash 去重
├── keyword_filter.py         # 关键词过滤
└── source_health.py          # 源健康监控
```

#### RSS 动态新增（M8 阶段）

WebUI 上能 CRUD RSS 源：
- 新增源（URL + 关键词 + 抓取频率 + 启用/停用）
- 查看源健康度（最近抓取时间、失败次数、抓取条数）
- 编辑源关键词
- 删除源（不会删历史数据）

存储：Redis KV `rss_sources:<id>` + InfluxDB 历史

⚠️ **只有非交易时段可以新增 / 修改源配置**（走交易时段守卫）

#### 舆情指数纳入决策融合（P3 之后）

```
原决策融合公式：
  finalBias = score × confidence × decay × alpha

新公式（P3 引入舆情后）：
  finalBias = score × confidence × decay × alpha + sentiment × beta

  其中：
    sentiment ∈ [-1, +1]：综合舆情指数
    beta = 0.1（舆情权重，比 AI 评分更轻）
```

#### 工作量分布

| 阶段 | 工作内容 | 周期 |
|---|---|---|
| **P1 (M4)** | domestic_intel + 财联社单源 + 简单情感分 | 1 周 |
| **P2 (M8)** | + 多源（股吧、雪球） + RSS 动态新增 UI + SimHash 去重 | 2 周 |
| **P3 (M11)** | + 香港跳板 + overseas_intel（4 个海外源） + 舆情指数纳入决策 | 2 周 |
| **V4 (M12+)** | + social_intel（Telegram 群组监听） | 2 周 |

✅ 总工程量：**7 周**（分散在 16 周内，不集中爆发）

---

### 机制 6：数据清洗管道与可追溯性（v2.0 新增）

> 「源头新闻和最终评分要关联上，要存下来，方便后期回测验证。」
> 这一节回答两个核心问题：①数据怎么清洗 ②如何做到从订单一路反查到原始 HTML

#### 五阶段清洗流水线

舆情数据从原始 HTML 到最终评分，**严格分 5 个阶段**，每个阶段输出落 InfluxDB（永久保存）：

```
原始 HTTP Response
   │
   ▼
[Stage 1: Fetch]         无 LLM     存 raw_intel bucket
   │
   ▼
[Stage 2: Parse]         无 LLM     存 parsed_intel bucket
   │
   ▼
[Stage 3: Clean]         可选轻量 LLM  存 cleaned_intel bucket
   │
   ▼
[Stage 4: Understand]    ⭐ LLM 上场   存 llm_analysis bucket
   │
   ▼
[Stage 5: Aggregate]     无 LLM     存 sentiment_index bucket
   │
   ▼
   trading-agent 决策融合使用
```

##### Stage 1: Fetch（抓取）— 不用 LLM

- 各 intel 服务（domestic/overseas/social）负责抓取
- **保留原始 HTTP response**（含 headers + body）
- 记录抓取时间、源 URL、HTTP 状态码
- ⚠️ 这一步 **不做任何内容处理**，只做"原汁原味存档"
- **为什么**：原始数据是审计的最后一道兜底，永远可以回头重新解析

##### Stage 2: Parse（解析）— 不用 LLM

- RSS 用 `feedparser`
- HTML 用 `BeautifulSoup` + 站点专用规则
- Telegram 用 `telethon` 内置 parser
- 提取结构化字段：标题 / 正文 / 发布时间 / 作者 / URL
- ⚠️ 这是"结构化"，不是"理解"
- **为什么不用 LLM**：规则引擎更快（毫秒级 vs 秒级）、更准（无幻觉）、零成本

##### Stage 3: Clean（清洗）— 部分用轻量 LLM

四步处理：

1. **文本归一化**（规则）：全角转半角、繁简转换、去 HTML 标签、去广告版权
2. **关键词过滤**（规则）：必须含"甲醇/MA/原油/煤炭/港口/产能"等；丢弃含"广告/招聘"等黑名单词
3. **跨源去重**（SimHash）：相同新闻不同源会被多次抓到，SimHash 距离 < 3 视为重复
4. **实体识别**（V2 加 LLM）：提取合约（MA2509/原油）、公司（Methanex）、地点、数字（库存量）

⚠️ V1 阶段实体识别用规则 + 词典，V2 才上 LLM（增量价值高时）

##### Stage 4: Understand（理解）— ⭐ LLM 完全负责

LLM 真正擅长的事：

```python
# 单条新闻 LLM 分析（伪代码）
prompt = f"""
分析这条新闻对甲醇期货的影响：

标题：{news.title}
内容：{news.body[:500]}

请输出 JSON：
{{
  "sentiment_score": float in [-1, 1],   # 看空~看多
  "confidence": float in [0, 1],          # 这条新闻的可靠度
  "event_types": [...],                   # 库存/价格/政策/事故/产能/物流
  "affected_symbols": ["MA2509", ...],    # 影响哪些合约
  "magnitude": "low" | "medium" | "high", # 影响幅度
  "reasoning": "..."                      # 简短归因（不超过 50 字）
}}
"""

response = llm.chat(prompt, model="deepseek-v3", temperature=0.1)
analysis = json.loads(response)
```

##### Stage 5: Aggregate（聚合）— 不用 LLM

合成时间窗口的舆情指数：

```python
def aggregate_sentiment(symbol, window_minutes=60):
    recent_analyses = query_recent(symbol, window_minutes)
    
    weighted_score = 0
    total_weight = 0
    contributing = []
    
    for a in recent_analyses:
        # 权重 = 源权威度 × 时效衰减 × LLM 置信度
        source_weight = SOURCE_AUTHORITY[a.source]  # 财联社=1.0, 雪球=0.5
        time_decay = exp(-(now - a.timestamp) / 3600)  # 1h 半衰期
        weight = source_weight * time_decay * a.confidence
        
        weighted_score += a.sentiment_score * weight
        total_weight += weight
        contributing.append({
            "news_id": a.cleaned_id,
            "weight": weight,
            "score": a.sentiment_score
        })
    
    return {
        "composite_score": weighted_score / total_weight,
        "contributing_news_ids": contributing,
    }
```

#### LLM 用与不用的原则

| 阶段 | 用 LLM? | 理由 |
|---|---|---|
| Fetch | ❌ | 纯网络 IO |
| Parse | ❌ | 规则引擎更快更准 |
| Clean | 🟡 V2 加 | 实体识别 LLM 有增量价值 |
| Understand | ✅ 完全用 | LLM 真正擅长的语义理解 |
| Aggregate | ❌ | 数学计算 |

⭐ **核心原则**：**LLM 只用在它擅长的事上**，其他用规则——省钱、快、可解释。

#### 可追溯性：6 层 ID 血缘体系

每一笔交易决策都能反查到当时影响它的原始 HTML。

```
news_raw_id        ─────┐
   │ = sha256(url + fetched_at)
   │
   ▼
news_parsed_id      ─── 同 raw_id（标识同一条新闻）
   │
   ▼
news_cleaned_id     ─── 同 parsed_id
   │
   ▼
news_analysis_id    ─── cleaned_id + ":" + llm_model_version
   │ ⚠️ 同一条新闻可能被 LLM 反复打分，需要版本号
   │
   ▼
sentiment_snapshot_id ─── sha256(symbol + timestamp_minute)
   │ contributing_news_ids: [analysis_id_1, analysis_id_2, ...]
   │ weights: [0.5, 0.3, ...]
   │
   ▼
ai_score_id          ─── score_uuid
   │ sentiment_snapshot_id: <上一层>
   │
   ▼
trade_decision_id    ─── decision_uuid
   │ ai_score_id: <上一层>
   │
   ▼
order_id             ─── decision_id: <上一层>
```

#### InfluxDB Bucket Schema（永久保存）

```
🟪 InfluxDB Buckets（全部永久保留，不设 TTL）

bucket: raw_intel
  measurement: news_raw
  tags:    source, fetcher_service
  fields:  raw_id, url, http_status, raw_html, fetched_at

bucket: parsed_intel
  measurement: news_parsed
  fields:  parsed_id (=raw_id), title, body, author, published_at, url

bucket: cleaned_intel
  measurement: news_cleaned
  tags:    source, has_keyword, dedup_status
  fields:  cleaned_id, title_clean, body_clean, keywords_matched,
           entities_json, simhash

bucket: llm_analysis ⭐ 关键
  measurement: news_analysis
  tags:    model_name, model_version, prompt_version
  fields:  analysis_id, cleaned_id, sentiment_score, confidence,
           event_types_json, affected_symbols_json,
           raw_llm_response, cost_tokens, latency_ms

bucket: sentiment_index ⭐ 关键
  measurement: index_minute
  tags:    symbol, window
  fields:  snapshot_id, composite_score,
           contributing_news_ids_json, weights_json

bucket: ai_score_history ⭐ 关键
  measurement: ai_score
  tags:    symbol, mode (live/shadow/emergency)
  fields:  score_id, score, confidence,
           sentiment_snapshot_id,
           technical_inputs_json, fundamental_inputs_json

bucket: trade_decisions ⭐ 关键
  measurement: decision
  tags:    symbol, action (buy/sell/hold)
  fields:  decision_id, ai_score_id, technical_signal,
           risk_check_passed, final_action

bucket: orders
  measurement: order
  tags:    symbol, status
  fields:  order_id, decision_id, ...
```

#### 反查能力示例

**场景**：某日 10:30 做空亏钱，想知道当时 trading-agent 依据什么决策。

```sql
-- 1. 找订单
SELECT * FROM orders WHERE timestamp = '10:30:15';
-- → decision_id: dec_yyy

-- 2. 找决策
SELECT * FROM trade_decisions WHERE decision_id = 'dec_yyy';
-- → ai_score_id: score_zzz

-- 3. 找 AI 评分
SELECT * FROM ai_score_history WHERE score_id = 'score_zzz';
-- → sentiment_snapshot_id: snap_aaa

-- 4. 找舆情快照
SELECT * FROM sentiment_index WHERE snapshot_id = 'snap_aaa';
-- → contributing_news_ids: [news_111, news_222, news_333]

-- 5. 反查每条新闻
FOR each news_id:
  SELECT * FROM llm_analysis WHERE cleaned_id = news_id;  -- LLM 分析
  SELECT * FROM news_cleaned WHERE cleaned_id = news_id;  -- 清洗后
  SELECT * FROM news_parsed WHERE parsed_id = news_id;    -- 结构化
  SELECT * FROM news_raw WHERE raw_id = news_id;          -- 原始 HTML
```

✅ **完整反查 6 层**，从订单一路追到原始 HTML。

#### Dashboard 决策溯源页面（P3 阶段）

```
┌──────────────────────────────────────────────────────────┐
│  决策溯源 — 订单 ord_2024061310300915                       │
├──────────────────────────────────────────────────────────┤
│  执行: 2026-06-13 10:30:15                                  │
│  动作: 做空 MA2509 1 手 @ 2350                              │
│                                                          │
│  └─决策依据 (decision_id: dec_yyy)                         │
│     ├─技术信号: MA 死叉 (强度 0.4)                          │
│     └─AI 评分: -0.65 (置信度 0.78)                          │
│        └─舆情快照 (snap_aaa) 贡献新闻 3 条:                   │
│            ├─📰 [财联社] 甲醇港口库存大幅累积                 │
│            │   情感分: -0.8  权重: 0.5                       │
│            │   [查看原文] [查看 LLM 分析]                    │
│            ├─📰 [雪球] 化工板块整体走弱                      │
│            │   情感分: -0.5  权重: 0.3                       │
│            │   [查看原文] [查看 LLM 分析]                    │
│            └─📰 [东方财富] WTI 原油下跌 2%                   │
│                情感分: -0.4  权重: 0.2                       │
└──────────────────────────────────────────────────────────┘
```

#### 时间倒带回测（关键设计）

回测时**绝不能再调 LLM**！

##### 错误做法（产生未来函数）：
```python
for historical_news in news_history:
    score = llm.analyze(historical_news)  # ❌ 用今天的 LLM 分析昨天的新闻
```

问题：LLM 模型可能升级，今天的 LLM 比当时的 LLM 更聪明 → 数据科学上叫"未来函数泄露"。

##### 正确做法（确定性回测）：
```python
for historical_news in news_history:
    # 直接用 InfluxDB 中【当时记录的】 LLM 输出
    score = query_influxdb('llm_analysis',
                            cleaned_id=historical_news.id)  # ✅
```

✅ 同样的历史数据跑两次结果完全一样（确定性）。

#### 存储成本预估

##### 数据量（甲醇 + 国内单源 V1）

| Bucket | 单条大小 | 日条数 | 年容量 |
|---|---|---|---|
| raw_intel | ~50 KB | 50 | 900 MB |
| parsed_intel | ~5 KB | 50 | 90 MB |
| cleaned_intel | ~3 KB | 30 | 32 MB |
| llm_analysis | ~2 KB | 30 | 21 MB |
| sentiment_index | ~1 KB | 1440 | 510 MB |
| ai_score_history | ~2 KB | 288 | 200 MB |
| trade_decisions | ~1 KB | 5 | 2 MB |
| **小计** | | | **~1.8 GB/年** |

##### P3 多源 + 海外源时

```
源数 1 → 8 → 数据量 × 8 ≈ 14 GB/年
单合约 → 多合约（× 3） ≈ 42 GB/年
```

✅ **100GB ESSD 数据盘够 2-3 年**（符合 P3 阶段加买数据盘的计划）。

##### LLM 调用成本

```
单条 LLM 分析：500 token 输入 + 200 token 输出
DeepSeek V3: ¥0.001 输入 + ¥0.0016 输出 = ~¥0.003/条

V1 单源：30 条/日 × 30 = 900 条/月 → ¥3/月  ✅
P3 多源：200 条/日 × 30 = 6000 条/月 → ¥18/月  ✅
```

✅ **舆情 LLM 成本极低**，远小于评分服务的 ¥250/月。

#### 工程要点（M4 实施时必须注意）

| 要点 | 说明 |
|---|---|
| **ID 必须穿透** | 5 层 bucket 必须用同一个 cleaned_id 关联，**不能丢失** |
| **LLM 输出原文必存** | `raw_llm_response` 字段保留 LLM 的原始 JSON，便于 debug |
| **prompt 版本号** | 每次改 prompt 升版本号，便于追溯历史评分 |
| **InfluxDB 永久不过期** | 这些 bucket 不设 TTL，是审计资产 |
| **跨服务透传 trace_id** | 用 OpenTelemetry 或简单的 trace_id 跨服务串联 |

---


## 资源预算与降级策略

> v1.7 加到 10 个服务后，4C8G 是临界状态。本节固化「先 4C8G 起步，远期升 8C16G」的策略和具体预算。

### 真实内存预估（v1.8 重新核算）

| 服务 | 语言 | P1 阶段 | P3 阶段（完整版） | 备注 |
|---|---|---|---|---|
| Redis 7.2（AOF + 1G maxmemory） | C | 1.0 GB | 1.2 GB | Stream 数据增长 |
| InfluxDB 2.7（**调小缓存到 512MB**） | Go | 2.0 GB | 2.5 GB | 写入压力增大 |
| ctp_tick | Go | 0.2 GB | 0.2 GB | |
| stock_tick (V2 占位) | - | 0 | - | V1 不启动 |
| indicator | Go | 0.2 GB | 0.2 GB | |
| llm_scorer | Python | 0.8 GB | 1.0 GB | |
| trading_agent | Go | 0.4 GB | 0.4 GB | |
| orchestrator_agent | Go | 0.3 GB | 0.4 GB | WebSocket 连接增多 |
| dashboard_web | Go | 0.2 GB | 0.2 GB | |
| watchdog | Go | 0.1 GB | 0.1 GB | |
| db_writer | Go | 0.3 GB | 0.4 GB | 批量缓冲 |
| **sentiment_monitor**（**P1 简版**：单源 + 5min） | Python | **0.3 GB** | 0.8 GB | P3 多源版本会涨 |
| 系统 + Docker daemon + 日志缓冲 | - | 1.3 GB | 1.5 GB | |
| **小计** | | **6.6 GB** | **8.9 GB** | |
| 4C8G 余量 | | 1.4 GB ✅ | -0.9 GB ⚠️ | |

### 决策记录

| 决策 | 内容 |
|---|---|
| 服务器配置 | **P1 起步 4C8G u2a 包年**，P2 末期评估是否升 8C16G |
| InfluxDB 缓存 | **调小到 512MB**（默认 1GB） |
| Python 服务内存上限 | docker-compose `mem_limit: 1G` 强制限制 |
| sentiment_monitor V1 范围 | **仅财联社化工 RSS，5 分钟一次**，单源单频 |
| Watchdog 内存告警 | **三级：80% / 90% / 95%**（钉钉推送，三级递进） |
| 升配触发条件 | Watchdog 长期（连续 7 天）报警内存 > 85%，或 P3 进入完整模式时 |

### Watchdog 内存告警三级机制

```go
// services/watchdog/internal/domain/memory_check.go
const (
    MemWarnLevel1 = 0.80  // 80%：黄色告警，钉钉提示
    MemWarnLevel2 = 0.90  // 90%：橙色告警，建议立即处理
    MemWarnLevel3 = 0.95  // 95%：红色告警，准备升配
)

// 告警去重：每个级别 1 小时内只推一次
// 级别下降时也推送（"内存恢复"）
```

### docker-compose.yml 内存限制

```yaml
services:
  llm_scorer:
    mem_limit: 1g
    mem_reservation: 768m
  
  sentiment_monitor:
    mem_limit: 512m
    mem_reservation: 256m
  
  influxdb:
    environment:
      INFLUXD_STORAGE_CACHE_MAX_MEMORY_SIZE: 512mb
    mem_limit: 2.5g
  
  redis:
    command: >
      redis-server
      --maxmemory 1gb
      --maxmemory-policy allkeys-lru
    mem_limit: 1.2g
```

### 升配路径（如果 P2/P3 内存吃紧）

| 触发场景 | 行动 |
|---|---|
| Watchdog 持续 7 天报 > 85% | 阿里云控制台升级到 8C16G（在线变配，约 ¥300/月增量） |
| Python 服务频繁 OOM | 先优化代码（Cython/缓存）+ 减小 batch 大小，**最后**才升配 |
| InfluxDB 写入卡顿 | 加买独立 100GB 数据盘，把 InfluxDB 数据迁出系统盘 |

### 月成本对照（更新版）

| 阶段 | 配置 | 月费 | 余量（1500 预算） |
|---|---|---|---|
| **P1 启动** | 4C8G u2a + 3M 带宽 + LLM API | ¥530-580 | ¥920+ |
| P2 中期（不升） | 同上 | ¥530-580 | ¥920+ |
| P2 末期（升 8C16G） | 8C16G + 3M + LLM | ¥780-880 | ¥620+ |
| **P3 完整** | 8C16G + 3M + 100GB ESSD + LLM + **香港代理 ECS 1C1G** | ¥930-1080 | ¥420+ |

✅ **最贵的方案仍在 1500 预算内**（P3 启用海外源时购买香港 ECS，约 ¥80-150/月）。

---

## 核心架构决策

### 部署形态：**全部上云**
原白皮书设计的"云陆分离"被简化为**单点云端部署**：
- ✅ Go 交易核心、Redis、InfluxDB、Dashboard、**Python AI Agent** 全部跑在阿里云 4C8G ECS 的 Docker 上
- ❌ 不要 Ollama 本地兜底（LLM 双双挂掉时直接停机告警）
- ❌ 不要 Mac 端常驻 Python（Mac 仅作为开发机 + 周末回测机）
- ✅ Tailscale 装但非关键链路（Mac 偶尔接入即可）

### 服务器配置：阿里云上海 4C8G ECS（u 系列通用算力型，包年）

| 项目 | 规格 | 月费估算（包年折算） |
|---|---|---|
| ECS 实例 | `ecs.u2a-c1m2.xlarge` 4C8G AMD（**或 u1 Intel 版本，二者价格相近、性能一致**；**避开突发型 t6/e 和共享型 s6**） | ¥150-230/月 |
| 系统盘 | 40GB ESSD PL0（默认） | 含 |
| 数据盘 | **延后购买**（M1 阶段先不买，触发线后再加挂载） | 0 → 后期 ¥30/月 |
| **公网 IP + 3M 固定带宽**（必选，不勾则连 CTP/LLM 都连不通） | 购买实例时勾选"分配公网 IPv4 地址" + 选**按固定带宽 3M** | ¥50/月（约 ¥600/年） |
| **基础设施小计** | | **¥200-280/月** |
| DeepSeek API（5min/次评分） | 输入 ¥2/M，输出 ¥8/M token | ¥250/月 |
| 智谱 GLM-4 API（备用熔断） | 备而不用 | ¥50/月 |
| **总计** | | **约 ¥500-580/月** |

⚠️ **公网 IP 决策记录**：
- 阿里云购买页"公网 IP" 默认不勾，**必须手动勾选** + 选"按固定带宽 3M"
- **不勾选的后果**：服务器没有公网出口，连 `apt update` 都跑不通，更别说连 CTP/SimNow 和 DeepSeek API
- **为什么不选弹性 EIP**：单独买 EIP 配置复杂、按量计费有不确定性；直接勾固定带宽最省心
- **为什么不选 1M**：CTP Tick 高峰期会丢包；3M 实测峰值不超过 1.5M，留足余量
- **为什么不选 5M+**：用不到，纯浪费

⚠️ **数据盘延后购买决策记录**：
- **M1 阶段不买数据盘**，仅保留 40GB 系统盘（够开发期+前 3-6 个月运行）
- **触发购买条件**（命中任一即立即买）：①系统盘使用率 > 60%；②InfluxDB 数据 > 10GB；③运行满 60 天
- **Watchdog 必须监控系统盘使用率**，命中触发线推钉钉告警
- 后期挂载流程详见 `docs/DEPLOY.md`（M5 阶段产出，含 mkfs/mount/fstab/Docker 数据迁移完整步骤）
- **数据盘必须选与实例相同的可用区**（跨区不能挂载）

⚠️ **采购确认事项**（购买前必须自己核实，因为阿里云价格随促销活动波动较大）：
1. 登录阿里云控制台，按上海地域 + u 系列（u1/u2a/u2i 任选） + 4C8G + 1 年期实际查价
2. 核对是否有"新人/学生/续费"优惠可叠加
3. 选择"包年"而非"按量"，节省 30-40%
4. 系统镜像选 **Ubuntu 22.04 LTS**（Docker 兼容性最好，CTP glibc 兼容性已验证）
5. **可用区手动选 F/G/L 区**（新区设备新、网络好），不要选"随机分配"
6. **登录名选 `ecs-user`**（阿里云 2025 年新增的安全推荐选项），不要用 root
7. **登录凭证选密钥对**（不要密码），私钥下载后 `chmod 600` 保管在 `~/.ssh/` 下

### LLM 熔断策略：二级 + 安全停机

```
DeepSeek-V3 (主)  →  智谱 GLM-4 (备)  →  停机 + 钉钉告警
                                              ↓
                                    Go 引擎切换"AI=0 模式"
                                    （只平仓不开新仓，等 LLM 恢复）
```

### AI 评分参与决策：MVP 就接入轻权重 + Shadow 双轨
- Redis 中**双 key**：`ai_score:MA2509:live`（参与下单，alpha=0.2）+ `ai_score:MA2509:shadow`（不参与，仅记录）
- 决策融合公式：`finalBias = score × confidence × decay × alpha`
- 老化机制（硬编码不可关闭）：`<15min: decay=1.0` / `15-30min: decay=0.5` / `>30min: decay=0.0`
- 仓位影响最大 ±20%（V1 安全值）

### 告警渠道：钉钉 Webhook
创建一个一人钉钉群，配置自定义机器人 webhook，所有告警（LLM 双挂、CTP 断连、AI 评分超时、强平触发、Docker 容器重启）统一推送。

---

## 项目目录结构

```
arbitrage-system/                   📍 monorepo 根（独立 git 仓库）
├── README.md
├── CLAUDE.md                       AI 协作规范（M1 阶段从老项目改写）
├── .gitignore
├── docker-compose.yml              ⭐ 多服务编排 + profiles 支持模块化启动
├── docker-compose.test.yml         集成测试用编排
├── .env.example                    环境变量模板（不入库 .env）
├── Makefile                        顶层快捷命令（gen / build-all / test / deploy）
│
├── contracts/                      ⭐⭐⭐ 跨服务通信契约（SSOT 唯一真相源）
│   ├── README.md
│   ├── buf.yaml / buf.gen.yaml     buf 工具链配置
│   ├── Makefile                    make gen / make lint / make breaking
│   │
│   ├── ai_score.proto              AI 评分（llm_scorer → trading-agent）
│   ├── tick.proto                  行情数据（ctp_tick → indicator/trading-agent）
│   ├── indicator.proto             技术指标（indicator → trading-agent）
│   ├── order.proto                 报单（trading-agent → broker）
│   ├── position.proto              持仓
│   ├── alert.proto                 告警（watchdog → 钉钉）
│   ├── healthz.proto               统一健康检查响应
│   ├── orchestrator.proto          ⭐ orchestrator gRPC 服务定义
│   │
│   └── generated/                  🤖 自动生成（不入 git）
│       ├── go/                     → 各 Go 服务引用
│       └── python/                 → llm_scorer 引用
│
├── services/                       ⭐ 所有微服务统一放在 services/ 下（v2.1 合并到 8 个）
│   │
│   ├── ctp_tick/                   📡【数据采集】CTP 行情订阅服务
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── README.md
│   │   ├── .env.example
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── domain/             Tick 标准化纯逻辑
│   │   │   ├── adapters/
│   │   │   │   ├── ctp/            cgo 封装 openctp/ctp-go
│   │   │   │   └── redis/          写 Redis Streams
│   │   │   └── api/                /healthz /metrics
│   │   └── tests/{unit,integration}/
│   │
│   ├── stock_tick/                 📡【数据采集】股票行情（V2 占位）
│   │   ├── README.md               说明：V2 阶段实现
│   │   └── .gitkeep
│   │
│   ├── intel_collector/            📡⭐【数据采集】v2.1 合并 — 三层舆情统一采集
│   │   ├── Dockerfile              python:3.12-slim
│   │   ├── pyproject.toml          uv 管理依赖
│   │   ├── README.md
│   │   ├── main.py                 调度入口（asyncio 多源并发）
│   │   ├── domain/
│   │   │   ├── fetchers/           按源类型抓取
│   │   │   │   ├── rss_fetcher.py
│   │   │   │   ├── html_fetcher.py
│   │   │   │   └── telegram_fetcher.py  (V4 加)
│   │   │   ├── parsers/            结构化提取
│   │   │   ├── cleaners/           归一化 + 关键词过滤
│   │   │   └── deduper.py          SimHash 跨源去重
│   │   ├── adapters/
│   │   │   ├── proxy.py            ⭐ 配置驱动是否走香港代理
│   │   │   ├── redis_writer.py
│   │   │   └── source_registry.py  ⭐ 动态 RSS 源管理（M8 加）
│   │   ├── config/
│   │   │   ├── domestic_sources.yaml  国内源（不走代理）
│   │   │   ├── overseas_sources.yaml  海外源（P3 走香港代理）
│   │   │   └── social_sources.yaml    社交源（V4 走代理）
│   │   ├── api/                    FastAPI /healthz /metrics
│   │   └── tests/
│   │
│   ├── llm_scorer/                 🤖【数据加工】LLM 评分服务
│   │   ├── Dockerfile              python:3.12-slim
│   │   ├── pyproject.toml          uv 管理依赖
│   │   ├── README.md
│   │   ├── main.py                 入口：定时调度
│   │   ├── domain/
│   │   │   ├── agents/             多 Agent 协作（market/technical/fundamentals/fuser）
│   │   │   ├── sentiment/          ⭐ 单条新闻情感打分（订阅 news_stream）
│   │   │   ├── prompt_templates.py
│   │   │   └── scoring.py
│   │   ├── adapters/
│   │   │   ├── llm_gateway.py      ⭐ 二级熔断（DeepSeek → GLM → 停机）
│   │   │   ├── data_fetcher.py     甲醇库存/基差/原油联动
│   │   │   ├── redis_writer.py     写 ai_score:MA2509:live + :shadow
│   │   │   └── config.py
│   │   ├── api/                    FastAPI /healthz /metrics + /emergency_analysis
│   │   └── tests/
│   │
│   ├── trading_agent/              🟢⭐【决策核心】v2.1 合并 — 含 indicator
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── README.md
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── domain/             ⭐ 业务逻辑（零外部依赖）
│   │   │   │   ├── indicator/      ⭐ v2.1 合入：MA / 布林带 / ATR / 量比
│   │   │   │   ├── strategy/       策略引擎 + ma_trend.go (V1)
│   │   │   │   ├── decision/       决策融合公式
│   │   │   │   ├── risk/           风控规则
│   │   │   │   └── position/       持仓管理
│   │   │   ├── adapters/           ⭐ 外部 IO 适配
│   │   │   │   ├── broker/
│   │   │   │   │   ├── interface.go  ⭐⭐ Broker 抽象接口
│   │   │   │   │   ├── ctp/          CTP 实现（V1）
│   │   │   │   │   └── (qmt/, easytrader/  V2 加)
│   │   │   │   ├── redis/          订阅 tick + 状态读写
│   │   │   │   └── influxdb/       订单流水
│   │   │   ├── api/
│   │   │   │   ├── grpc/           ⭐ gRPC :9090（暴露给 orchestrator）
│   │   │   │   ├── healthz.go      :8081/healthz
│   │   │   │   └── metrics.go      :8081/metrics
│   │   │   └── app/wire.go         装配层
│   │   └── tests/
│   │
│   ├── orchestrator/               🟦⭐【编排层】v2.2 — 不再含 dashboard
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── README.md
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── domain/             状态聚合 + 指令路由（纯逻辑）
│   │   │   ├── adapters/
│   │   │   │   ├── trading_client/ ⭐ gRPC 客户端 → trading-agent
│   │   │   │   ├── redis/          只读状态聚合
│   │   │   │   └── audit/          操作审计日志
│   │   │   └── api/
│   │   │       ├── rest/           ⭐ REST :8080/api/v1/*
│   │   │       ├── websocket/      ⭐ WS :8080/ws
│   │   │       ├── healthz.go
│   │   │       └── metrics.go
│   │   └── tests/
│   │
│   ├── dashboard_web/              🟨⭐【表现层】v2.2 — Next.js + Tremor + shadcn/ui
│   │   ├── Dockerfile              多阶段构建（Node builder + nginx 托管静态资源）
│   │   ├── package.json
│   │   ├── tsconfig.json
│   │   ├── tailwind.config.ts
│   │   ├── next.config.js
│   │   ├── README.md
│   │   ├── public/                 字体 + 图标
│   │   ├── src/
│   │   │   ├── app/                Next.js App Router（16 个页面）
│   │   │   ├── components/         ⭐ shadcn/ui + Tremor + 自定义组件
│   │   │   │   ├── ui/             shadcn/ui 复制粘贴的组件
│   │   │   │   ├── charts/         Tremor 图表封装
│   │   │   │   ├── cards/          KPI 卡片
│   │   │   │   ├── layout/         侧边栏 + 顶栏 + 主框架
│   │   │   │   └── shared/         紧急按钮等
│   │   │   ├── lib/
│   │   │   │   ├── api.ts          orchestrator REST 客户端
│   │   │   │   └── ws.ts           WebSocket 连接管理
│   │   │   ├── hooks/              自定义 React hooks
│   │   │   ├── stores/             Zustand 全局状态
│   │   │   ├── types/              TypeScript 类型（从 Protobuf 生成）
│   │   │   └── styles/
│   │   │       ├── globals.css     Tailwind base + 自定义变量
│   │   │       └── tokens.ts       ⭐ 设计令牌（颜色 / 字体 / 效果）
│   │   └── tests/{e2e, unit}/
│   │
│   ├── db_writer/                  🟪【持久化】InfluxDB 串行写入
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── README.md
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── domain/             批量聚合规则
│   │   │   ├── adapters/
│   │   │   │   ├── redis/          消费 Redis Streams
│   │   │   │   ├── influxdb/       批量写入
│   │   │   │   └── dlq/            死信队列处理
│   │   │   └── api/                /healthz /metrics
│   │   └── tests/
│   │
│   └── watchdog/                   🟧【监控告警】健康监控
│       ├── Dockerfile
│       ├── go.mod
│       ├── README.md
│       ├── cmd/main.go
│       ├── internal/
│       │   ├── domain/             健康判定规则
│       │   └── adapters/
│       │       ├── healthz_client.go    巡检各服务
│       │       ├── disk_monitor.go      磁盘使用率
│       │       ├── memory_monitor.go    内存使用率（v1.8 三级告警）
│       │       └── dingtalk.go          钉钉 Webhook
│       └── tests/
│
├── scripts/                        🛠 运维脚本（不属于任何微服务）
│   ├── init_server.sh              ⭐ 服务器首次登录一键加固
│   ├── init_redis.sh
│   ├── seed_data.sh
│   ├── smoke_test.sh               ⭐ E2E 冒烟测试
│   └── deploy.sh
│
└── docs/
    ├── ARCHITECTURE.md             ✅ 本文档
    ├── CONTRACTS.md                📝 待写（M1 末期）
    ├── DEPLOY.md                   📝 待写（M5）
    └── TROUBLESHOOT.md             📝 待写（M6）
```

### 为什么把所有微服务放在 `services/` 下？

- **monorepo 内部清晰分层**：`contracts/`（共享）、`services/`（运行时）、`scripts/`（运维）、`docs/`（文档）
- **新增服务零思考**：未来加股票模块时，直接 `services/stock_xxx/` 即可
- **CI 友好**：`services/<name>/` 路径变化触发对应服务构建

### docker-compose.yml profiles 设计（v2.2 更新）

```yaml
services:
  redis:
    profiles: ["full", "ai-only", "futures", "ui"]
  influxdb:
    profiles: ["full", "futures", "ui"]
  ctp_tick:
    profiles: ["full", "futures"]
  intel_collector:
    profiles: ["full", "ai-only"]
  llm_scorer:
    profiles: ["full", "ai-only"]
  trading_agent:                  # 已含 indicator
    profiles: ["full", "futures"]
  orchestrator:                   # 不含 dashboard（v2.2 拆分）
    profiles: ["full", "ui", "futures"]
  dashboard_web:                  # ⭐ v2.2 独立 Next.js 服务
    profiles: ["full", "ui"]
  db_writer:
    profiles: ["full", "futures", "ai-only"]
  watchdog:
    profiles: ["full"]
```

启动方式：
```bash
docker compose --profile full up      # 生产 / 完整启动（9 个容器）
docker compose --profile futures up   # 期货侧：redis + influxdb + ctp_tick + trading_agent + orchestrator + db_writer
docker compose --profile ai-only up   # AI 侧：redis + intel_collector + llm_scorer + db_writer
docker compose --profile ui up        # 前端侧：redis + influxdb + orchestrator + dashboard_web
```

---

## 服务发现与启动协调

> 「节点之间怎么知道对方存在？挂了怎么重连？启动顺序怎么保证？」—— 这一节回答这些问题。

### 服务发现策略：静态配置 + Docker DNS

**不引入 Consul/Etcd/Nacos 等注册中心**。8 个服务、单机 Docker Compose、单实例部署，**纯过度设计**。

直接用 Docker Compose 内置能力：
1. **Docker Compose 自动建内部网络**（`arbitrage-system_default`）
2. **服务名 = DNS hostname**：`redis` 这个域名直接解析为 Redis 容器的内部 IP
3. **每个服务的 `.env` 里写死对方地址**：`REDIS_ADDR=redis:6379`、`TRADING_AGENT_GRPC=trading_agent:9090`
4. 容器重启 IP 变了不影响——**Docker DNS 自动更新**

### 配置示例

```bash
# services/trading_agent/.env.example
SERVICE_NAME=trading_agent
HEALTH_PORT=8081
GRPC_PORT=9090

REDIS_ADDR=redis:6379
INFLUXDB_URL=http://influxdb:8086

CTP_FRONT_ADDR=tcp://180.168.146.187:10130
CTP_BROKER_ID=9999
# ...
```

```bash
# services/orchestrator_agent/.env.example
SERVICE_NAME=orchestrator_agent
HTTP_PORT=8080

TRADING_AGENT_GRPC=trading_agent:9090   # ⭐ Docker DNS 解析
REDIS_ADDR=redis:6379

# 巡检列表（健康监控用）
HEALTHZ_TARGETS=ctp_tick:8081,indicator:8081,llm_scorer:5000,trading_agent:8081
```

```bash
# services/watchdog/.env.example
HEALTHZ_TARGETS=ctp_tick:8081,indicator:8081,llm_scorer:5000,trading_agent:8081,orchestrator_agent:8080
SCRAPE_INTERVAL=5s
```

### 启动顺序保证

`depends_on` 只管启动顺序，不管"是否真的可用"。两道防线：

#### 第一道：Docker 健康检查门槛

```yaml
services:
  redis:
    image: redis:7.2
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  trading_agent:
    depends_on:
      redis:
        condition: service_healthy   # ⭐ 等 Redis 真的健康了才启动
      influxdb:
        condition: service_healthy
```

#### 第二道：客户端启动重试（必备）

每个服务连依赖时**必须有重试逻辑**，不能"连不上就退出"：

```go
// 伪代码
func connectWithRetry(addr string, timeout time.Duration) *Client {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        c, err := connect(addr)
        if err == nil {
            return c
        }
        log.Warn("waiting for", addr)
        time.Sleep(2 * time.Second)
    }
    log.Fatal("cannot connect to", addr)
}
```

### 运行时断线重连

**所有客户端必须开启自动重连**：

| 客户端 | 重连方式 |
|---|---|
| Redis（go-redis） | `MaxRetries: -1` 内置无限重试 |
| InfluxDB（client v2） | 内置 retry policy + 指数退避 |
| gRPC | `grpc.WithDefaultServiceConfig(retryPolicy)` |
| HTTP（healthz/REST） | 自定义指数退避：1s/2s/4s/8s ... 最大 30s |
| CTP（openctp） | `OnFrontDisconnected` 回调中触发自动重连 |

### 轻量级服务注册（用 Redis 做注册表）

虽然不需要 Consul，但**让 orchestrator 知道"当前哪些服务在线"**仍有价值——给 Dashboard 显示用。

#### 协议

每个服务启动时：
1. **注册自己**到 Redis：`SET services:<name> <info> EX 30`
2. **定时续约**：每 10 秒 `EXPIRE services:<name> 30`
3. 服务挂了 → 30 秒内 TTL 过期 → 自动从注册表消失

```go
// 每个服务启动时调用
type ServiceInfo struct {
    Name         string   `json:"name"`           // "trading_agent"
    Version      string   `json:"version"`        // "v0.1.3"
    StartedAt    int64    `json:"started_at"`     // unix ms
    HealthURL    string   `json:"health_url"`     // "http://trading_agent:8081/healthz"
    GRPCAddr     string   `json:"grpc_addr"`      // "trading_agent:9090"（如有）
    Capabilities []string `json:"capabilities"`   // ["futures", "ctp"]
}

func RegisterAndKeepAlive(rdb *redis.Client, info ServiceInfo) {
    key := "services:" + info.Name
    payload, _ := json.Marshal(info)

    rdb.Set(ctx, key, payload, 30*time.Second)

    go func() {
        ticker := time.NewTicker(10 * time.Second)
        for range ticker.C {
            rdb.Expire(ctx, key, 30*time.Second)
        }
    }()
}
```

#### orchestrator 查询在线服务

```go
keys, _ := rdb.Keys(ctx, "services:*").Result()
for _, key := range keys {
    info, _ := rdb.Get(ctx, key).Result()
    // 渲染到 Dashboard
}
```

### 关键决策记录

| 决策 | 选择 | 理由 |
|---|---|---|
| 服务发现 | **Docker Compose DNS + 环境变量** | 单机部署，过度设计无意义 |
| 注册中心 | **不引入** | 用 Redis KV+TTL 做轻量注册即可 |
| 启动顺序 | **健康检查 + 客户端重试** | 双保险，不依赖单一机制 |
| 断线重连 | **客户端原生重连机制** | 所有主流客户端都支持，零额外代码 |
| 配置管理 | **`.env` 文件 + Viper** | 不引入 Consul KV、Apollo 等 |

---

## 阶段性实施路线（架构全量 + 实施分 3 阶段，共 18 周）

> **v2.3 调整**：因采用 Vision UI 开源模板，Dashboard 工作量从 5 周降到 3 周。
> P1 核心 11 周 / P2 稳定 4 周 / P3 完整 3 周 = 18 周。

### 🥉 P1：核心闭环（第 1-11 周，含 3 周 Dashboard 开发）

**P1 目标**：跑通端到端最小版本 + 高级 Dashboard，SimNow 模拟盘连续 3 天稳定运行。

### M1 — 基础设施搭建（第 1-2 周）

**目标**：阿里云 ECS 就绪，安全加固完成，Docker 编排跑通，所有基础服务在线，**契约工具链就绪**。

**交付物**：
- 阿里云 ECS 4C8G u1 实例已购买（含公网 IP + 3M 固定带宽），安装 Ubuntu 22.04
- ⭐ **`scripts/init_server.sh` 一键安全加固脚本**（首次登录服务器后执行，详见下面"服务器初始化与安全加固"章节）
- 阿里云安全组按"白名单原则"配置（仅放行 SSH 自定义端口 + Tailscale）
- Docker + Docker Compose 安装
- `docker-compose.yml` 编排：Redis 7.2（含 AOF 持久化）+ InfluxDB 2.7 + 10 个空壳服务容器（ctp_tick / indicator / llm_scorer / trading_agent / orchestrator_agent / dashboard_web / watchdog / **db_writer** / **sentiment_monitor**；stock_tick 仅占位），**带 profiles 支持模块化启动**
- ⭐ **`contracts/` 目录初始化**：buf 工具链 + 7 个 .proto 文件骨架（ai_score / tick / order / position / alert / healthz / orchestrator）+ `make gen` 命令验证可生成 Go/Python 代码
- Tailscale 安装在 ECS 和 Mac 上（MagicDNS 启用，主机名 `ali-trade`）
- `git init` + GitHub 私有仓库（待用户确认仓库托管平台）
- 钉钉机器人 webhook 已配置并测试通过

**验证**：
- ECS 上 `docker compose --profile full up` 全绿
- Mac 上 `ping ali-trade` 通
- SSH 22 端口对公网关闭，自定义端口（如 22922）只允许密钥登录
- ⭐ `cd contracts && make gen` 成功生成 Go 和 Python 代码
- 钉钉群收到测试消息

### M2 — CTP 行情接入 + Tick 入库（第 3-4 周）

**目标**：`ctp_tick` 服务连 SimNow，订阅甲醇主力，Tick 入 Redis Streams + InfluxDB。

**交付物**：
- `services/ctp_tick/adapters/ctp/md.go`：CTP 行情订阅（cgo 封装 `openctp/ctp-go`）
- `services/ctp_tick/adapters/redis/`：写 Redis Streams `tick:MA2509`
- `services/ctp_tick/adapters/influxdb/`：批量写 `ma_tick` bucket
- `services/ctp_tick/Dockerfile`：多阶段构建（Linux 容器内 cgo 编译，输出静态二进制）
- `Makefile`：`make build-ctp-tick` / `make test-unit`

**验证**：
- 日志显示 `OnFrontConnected` + `SubscribeMarketData("MA")` 成功
- `XLEN tick:MA2509` 显示 Streams 中有 Tick 推送
- `influx query 'from(bucket:"ma_tick") |> range(start:-1h) |> count()'` 返回 > 1000

**关键风险**：
- ⚠️ **CTP cgo 在 macOS 上编译不通过**（依赖 Linux .so）— 解决方案：`ctp_tick` 服务的 CTP 相关代码加 `//go:build ctp` build tag，本地用 mock 实现，Docker 内编译真版本
- ⚠️ **SimNow 凭证申请要 1-3 工作日** — 用户**今天就要去 simnow.com.cn 申请**，不阻塞 M1

### M3 — 数据采集 + 决策核心（trading-agent）（第 5-6 周，与 M4 并行）

**目标**：`ctp_tick` + `indicator` + `trading-agent` 三个 Go 服务跑通，能算指标、出信号、下单。

**交付物**：
- `services/ctp_tick/`：CTP 行情订阅 → Redis Streams（cgo 封装）
- `services/indicator/`：订阅 tick → 算指标 → 写 Redis KV
- `services/trading_agent/`：
  - `domain/strategy/ma_trend.go`：MA 金叉/死叉策略
  - `domain/decision/fuser.go`：技术信号 + AI 评分融合
  - `domain/risk/manager.go`：风控规则
  - `adapters/broker/ctp/`：CTP 报单实现
  - `api/grpc/`：暴露 gRPC :9090（暂时不接前端）

**验证**：
- SimNow 收到 `OnRtnOrder` + `OnRtnTrade` 回报
- 风控测试：人为触发超限，日志显示 `RISK_BLOCK`
- AI 评分老化测试：手动写入 31min 前的评分，trading-agent 切"AI=0 模式"
- gRPC 测试：`grpcurl -plaintext localhost:9090 list` 列出服务

### M4 — LLM 评分服务（llm_scorer）（第 5-6 周，与 M3 并行）

**目标**：`llm_scorer` Python 服务每 5 分钟生成甲醇 AI 评分写 Redis。

**交付物**：
- `services/llm_scorer/domain/agents/`：四个 Agent
- `services/llm_scorer/adapters/llm_gateway.py`：DeepSeek → GLM-4 → 停机
- `services/llm_scorer/adapters/redis_writer.py`：写 `ai_score:MA2509:live` + `:shadow`

**验证**：
- `redis-cli get ai_score:MA2509:live` 返回完整 protobuf 解码后的 JSON
- LLM 熔断测试：DeepSeek key 改错 → 自动切 GLM
- 双挂测试：两个 key 都改错 → 钉钉告警 + Redis 写"停机"标志

### M5 — orchestrator + watchdog（第 7 周）

**目标**：`orchestrator` + `watchdog` 上线，REST/WebSocket 接口就绪，**等待 dashboard 接入**。

**交付物**：
- `services/orchestrator/`：
  - `adapters/trading_client/`：gRPC 客户端连 trading-agent
  - `api/rest/`：REST :8080/api/v1/*
  - `api/websocket/`：WS :8080/ws 实时推送
- `services/watchdog/`：巡检 healthz + 磁盘监控 + 内存监控 + 钉钉
- 临时 curl/Postman 测试 orchestrator API

**验证**：
- `curl http://orchestrator:8080/api/v1/overview` 返回完整状态 JSON
- WebSocket 客户端测试工具能收到 snapshot/delta/event 推送
- 一键平仓 API：`curl -X POST http://orchestrator:8080/api/v1/emergency_close`
- 主动 kill `trading-agent` 容器 → watchdog 30 秒内推钉钉

### M6 — Dashboard 基于 Vision UI 改造（第 8-10 周，3 周）⭐ 重头戏

**目标**：基于 Vision UI 模板改造，金融科技深色风。

**Sprint 0（1 天）**：脚手架
- Clone Vision UI Dashboard React 到 `services/dashboard_web/`
- 改 package.json 名称、删除 demo 数据、删除不用的页面
- 装 Tremor + TailwindCSS（仅作 Tremor 依赖）
- 装 TanStack Query + Zustand
- 保留原 LICENSE_VISION_UI.md（MIT 协议要求）

**Sprint 1（3 天）**：基础设施
- API 客户端（TanStack Query 包装 axios）
- WebSocket 连接管理（自动重连 + snapshot 兜底）
- Zustand 全局状态（系统状态 / 交易状态）
- 类型定义（基于 Protobuf 生成 JS proto 客户端）

**Sprint 2（3 天）**：⭐ **概览页改造**
- 基于 Vision UI 仪表盘模板（已有渐变折线 + 圆环 + KPI 卡片）
- 替换 demo 数据为 orchestrator API
- 系统状态卡片（9 个服务的健康灯）
- 实时盘面卡片（Tremor AreaChart 替换 ApexCharts，更专业）
- AI 评分卡片（Tremor ProgressCircle 带光晕）
- KPI 卡片（持仓 / 浮动盈亏 / 今日报单 / 今日 PnL）
- 最近事件流（实时 WebSocket 推送 + 滚动列表）

**Sprint 3（3 天）**：交易相关页面
- 持仓页（基于 Vision UI 表格模板）
- 委托页（含分页 + 筛选）
- 成交页
- 全局紧急平仓按钮（MUI Dialog 二次确认）

**Sprint 4（2 天）**：监控页
- 节点状态详情（基于 Vision UI 状态卡片）
- 主机性能（CPU/内存/磁盘 实时图表）

**Sprint 5（2 天）**：AI 相关页
- AI 评分详情（live + shadow 对比）
- 评分历史曲线（Tremor LineChart）

**Sprint 6（3 天）**：联调 + 优化
- 与 orchestrator 全链路对接
- 性能优化（虚拟滚动 + WebSocket 节流）
- 浏览器兼容测试
- 移动端适配

**验证**：
- 浏览器访问 `http://ali-trade:8000` 看到完整 Dashboard，**视觉效果接近用户参考图**
- 一键平仓全链路 OK
- WebSocket 实时推送：盘面 1 秒级刷新
- 手机浏览器响应式正常

### M7 — 冒烟测试 + 三日运行验收（第 11 周）

**目标**：全链路在 SimNow 连续无故障运行 3 个交易日。

**交付物**：
- `scripts/smoke_test.sh`：10 步 E2E 测试脚本
- 3 个交易日运行日志 + 交易记录归档
- `docs/DEPLOY.md` 部署手册
- `docs/TROUBLESHOOT.md` 排障指南

**验证**：
- CTP 每日 15:15 收盘断线、次日 9:00 前自动重连
- AI 评分每 5 分钟 ±10s 准时生成
- 策略产生 1-3 次报单/日
- 钉钉群收到完整告警
- Dashboard 连续 3 天 0 故障

---

### 🥈 P2：稳定性增强（第 12-15 周，2 个里程碑）

**P2 目标**：把核心机制 4（事件驱动 AI 紧急触发）+ 交易时段守卫 + 灰度发布加上。

#### M7 — 事件驱动 AI 介入（第 9-10 周）

**目标**：trading-agent 检测剧烈波动 → llm_scorer 紧急分析 → 谨慎模式响应。

**交付物**：
- trading-agent 单向波动监控 + 防抖（5 分钟内最多触发一次）
- llm_scorer 紧急分析 endpoint（专用 Prompt，30s timeout）
- emergency 评分写入 Redis（TTL 10 分钟）
- trading-agent 谨慎模式行为：暂停开新仓 / 加紧止损 / 仓位减半（用户可配）
- 紧急告警钉钉推送

**验证**：
- 人为构造剧烈波动 Tick（>1%/分钟）→ trading-agent 触发紧急分析
- 5 分钟内重复构造 → 防抖生效，不重复触发
- llm_scorer 收到紧急请求 30 秒内返回评分
- trading-agent 收到 emergency 评分 → 切谨慎模式 → 推钉钉

#### M8 — 交易时段守卫 + 策略灰度 + Dashboard 增强（第 11-12 周）

**目标**：策略管理流程严格化 + Dashboard 多页面。

**交付物**：
- orchestrator REST 中间件：`TradingSessionGuard` 拦截修改型操作
- 中国期货交易日历（甲醇 4 个时段）+ 节假日清单
- 策略 mode（shadow ↔ live）切换 API
- 策略版本历史（InfluxDB 永久保存）
- Dashboard 增加：
  - 节点状态详情页（每个服务的健康度 + 关键指标）
  - AI 评分历史曲线
  - 主机性能监控（CPU/内存/磁盘）

**验证**：
- 交易时段（如 10:00）尝试修改策略 → 返回 403 + 提示下次可用时间
- 非交易时段（如 18:00）→ 通过
- shadow 策略：生成信号但不下单，写 InfluxDB 记录
- shadow ↔ live 切换 → 必须在非交易时段，记录到审计日志

---

### 🥇 P3：完整版（第 16-18 周，3 个里程碑）

**P3 目标**：Dashboard 完整化、WebUI 策略管理、舆情扩展。

#### M9 — Dashboard 完整版（第 13-14 周）

**目标**：图表 + 完整页面。

**交付物**：
- 分时图（ECharts，当日 Tick 聚合 1 秒级）
- 日 K / 周 K 图（从 InfluxDB ma_1min 聚合）
- 对账单页面（按日 / 按月汇总）
- 期货当日委托 + 成交明细页
- 操作审计日志页

**验证**：
- 浏览器看到完整分时图 + K 线图
- 对账单和 SimNow 后台数字对得上

#### M10 — WebUI 策略管理（第 15 周）

**目标**：策略不再硬编码，可通过 WebUI 上传/编辑 YAML。

**交付物**：
- YAML 编辑器（Monaco Editor 嵌入 HTML）
- 策略列表 / 编辑 / 启停 / shadow ↔ live 操作
- 风控参数管理 UI
- 全部操作走交易时段守卫

**验证**：
- 非交易时段：能新建/编辑/启用策略
- 交易时段：所有修改型操作被拒
- 策略版本回滚正常

#### M11 — 舆情扩展（第 16 周）

**目标**：sentiment_monitor 从 V1 最简版扩展到 V2 多源版。

**交付物**：
- sentiment_monitor 增加数据源：雪球甲醇话题 + 卓创资讯（如有订阅）
- LLM 关键事件提取（库存/政策/事故/产能）
- 舆情指数纳入决策融合公式：`+ sentiment × beta（beta=0.1）`
- Dashboard 舆情看板页

**验证**：
- 多源数据正常入流
- 关键事件能正确分类
- 舆情指数纳入决策后，shadow 模式对比验证有效性

---

## Dashboard 页面规划

> Dashboard 不是一个页面，而是一组页面。按 P1/P2/P3 阶段渐进交付。

### 页面清单（按阶段，v2.2 P1 升级版）

| 页面 | 路由 | P1 | P2 | P3 | 备注 |
|---|---|---|---|---|---|
| **概览** | `/` | ✅ 高级 | ✅ | ✅ | 含渐变折线 + 圆环 + 光晕效果（参考用户原型） |
| **节点状态** | `/nodes` | ✅ | ✅ | ✅ | 各服务健康度 + 关键指标 |
| **主机性能** | `/host` | ✅ | ✅ | ✅ | CPU/内存/磁盘/网络 实时图表 |
| **期货 - 持仓** | `/futures/positions` | ✅ | ✅ | ✅ | 持仓 + 浮动盈亏可视化 |
| **期货 - 委托** | `/futures/orders` | ✅ | ✅ | ✅ | 当日委托 + 历史筛选 |
| **期货 - 成交** | `/futures/trades` | ✅ | ✅ | ✅ | 当日成交明细 |
| **期货 - 分时图** | `/futures/intraday` | ❌ | ✅ | ✅ | TradingView Lightweight Charts |
| **期货 - K 线** | `/futures/kline` | ❌ | ✅ | ✅ | 日 K / 周 K（TradingView） |
| **期货 - 对账单** | `/futures/statement` | ❌ | 简版 | ✅ | 按日 / 按月汇总 |
| **AI 评分** | `/ai/scores` | ✅ | ✅ | ✅ | live + shadow + 老化时间 |
| **AI 评分历史** | `/ai/scores/history` | ✅ | ✅ | ✅ | 历史曲线 |
| **舆情看板** | `/sentiment` | ❌ | ❌ | ✅ | 新闻流 + 情感分布 |
| **决策溯源** | `/audit/decisions/[id]` | ❌ | ❌ | ✅ | v2.0 6 层 ID 反查（订单 → 新闻） |
| **策略管理** | `/strategies` | 只读 | 启停 | ✅ 完整 | Monaco YAML 编辑 + 灰度 |
| **风控参数** | `/risk` | 只读 | ✅ | ✅ | 修改要走时段守卫 |
| **审计日志** | `/audit` | ✅ 简版 | ✅ | ✅ | 所有修改型操作 |
| **股票相关页面** | `/stocks/*` | ❌ | ❌ | ❌ | V2 加股票模块时做 |
| **紧急平仓** | 全局按钮 | ✅ | ✅ | ✅ | 二次确认 + 任何时段可用 |

### P1 概览页设计（参考用户提供的金融科技深色风原型）

P1 的核心目标是验证链路通，不是好看：

```
┌──────────────────────────────────────────────────────────┐
│ AI Trading System                          [紧急平仓] 🔴   │
├──────────────────────────────────────────────────────────┤
│ 系统状态                                                   │
│  🟢 ctp_tick      🟢 indicator   🟢 trading_agent          │
│  🟢 llm_scorer    🟢 orchestrator 🟢 dashboard_web         │
│  🟢 watchdog      🟢 db_writer    🟡 sentiment_monitor     │
│                                                          │
│ 当前盘面: MA2509  价格 2350 (+0.5%)                        │
│ 当前持仓: 多 1 手 @ 2345  浮动盈亏 +50                      │
│                                                          │
│ AI 评分                                                   │
│  Live: +0.62 (置信度 0.78)                                 │
│  Shadow: +0.45                                            │
│  最后更新: 3 分钟前                                         │
│  老化系数: 1.0                                             │
│                                                          │
│ 今日:                                                     │
│  报单 2 次   成交 2 次   PnL +234                          │
│                                                          │
│ 最近事件:                                                 │
│  10:32 [INFO]  报单成交 MA2509 买开 1 手 @ 2345            │
│  10:15 [INFO]  AI 评分更新 +0.62                           │
│  09:55 [WARN]  CTP 重连成功                                │
└──────────────────────────────────────────────────────────┘
```

### Dashboard 技术栈（v2.3 — 基于 Vision UI 模板）

> v2.3 重大变更：放弃从零搭建 Next.js + Tremor 方案，**直接 fork Creative Tim 的 Vision UI Dashboard React**（MIT 开源）作为起点，节省 1-2 周纯 UI 开发时间。
> 仓库：https://github.com/creativetimofficial/vision-ui-dashboard-react
> License：MIT（免费商用）

#### 技术栈（v2.3 混合方案）

| 工具 | 角色 | 来源 |
|---|---|---|
| **Create React App (CRA)** | 项目脚手架 | Vision UI 自带 |
| **React 17** | UI 框架 | Vision UI 自带（不主动升级 18，避免兼容性踩坑） |
| **JavaScript** | 主语言 | Vision UI 自带（**不强加 TypeScript**，保持一致） |
| **Material-UI v5** | 通用组件库 | Vision UI 核心 |
| **Emotion** | CSS-in-JS | MUI 默认 |
| **React Router v5** | 路由 | Vision UI 自带 |
| **Tremor** ⭐ | **金融图表（趋势、KPI 圈、漏斗、占比）** | 新增 |
| **TradingView Lightweight Charts** ⭐ | **K 线图、分时图（专业级）** | 新增 |
| **ApexCharts** | 部分模板自带的高级图表（保留） | Vision UI 自带 |
| **TailwindCSS** | 仅作 Tremor 依赖装上 | 新增 |
| **TanStack Query** | API 数据管理 | 新增（替换 Vision UI 的 axios 直调） |
| **Zustand** | 全局状态 | 新增（替换 Vision UI 的 Context） |
| **socket.io-client** 或原生 WebSocket | 实时推送 | 新增 |
| **Monaco Editor** | YAML 编辑器（M10 用） | 新增 |

#### 三种图表的分工（重要）

| 图表类型 | 用什么 | 理由 |
|---|---|---|
| **K 线图、分时图** | **TradingView Lightweight Charts** | 行业标杆，性能好，金融专业（必选） |
| **趋势折线、面积图、KPI 圆环** | **Tremor** | 金融科技深色风开箱即用 |
| **漏斗图、桑基图、热力图** | ApexCharts（保留 Vision UI 模板的） | Tremor 不支持，且 Vision UI 模板已写好 |
| **简单数字卡片、表格** | MUI 原生组件 | 与 Vision UI 风格统一 |

#### 视觉一致性原则

**容器（Card）始终用 MUI/Vision UI 的**，**只把图表替换成 Tremor 或 TradingView**：

```tsx
// 正确做法 ✅
import { Card } from "@mui/material";          // 容器用 Vision UI 风格
import { AreaChart } from "@tremor/react";      // 图表用 Tremor

<Card sx={visionUICardStyle}>
  <Typography>实时盘面</Typography>
  <AreaChart 
    data={tickData}
    index="time"
    categories={["price"]}
    colors={["cyan", "indigo"]}
    showAnimation
    showGradient
  />
</Card>

// 不推荐 ❌（两套样式系统冲突）
import { Card } from "@tremor/react";           // 别用 Tremor 的容器
```

#### Vision UI 模板优势

直接拥有的 30+ 个页面模板：
- ✅ 概览仪表盘（带渐变折线 + 圆环 + KPI 卡片，**和你给我的设计原型几乎一模一样**）
- ✅ 表格 / 列表
- ✅ 表单
- ✅ 通知 / 弹窗
- ✅ 用户管理
- ✅ 计费 / 账单（可改造为对账单）
- ✅ 完整的设计令牌（颜色 / 字体 / 玻璃拟态效果）

只需要**裁剪不要的页面 + 接入 orchestrator API + 加 K 线图**。

#### 集成方式：直接复制代码（推荐）

```bash
# 在项目根目录
cd /Users/jacklee/myProject/arbitrage-system/services/

# 克隆 Vision UI 到临时目录
git clone https://github.com/creativetimofficial/vision-ui-dashboard-react /tmp/vision-ui

# 复制源码到我们的项目（保留 LICENSE 和 README 中的版权声明）
cp -r /tmp/vision-ui/* dashboard_web/
cp /tmp/vision-ui/LICENSE.md dashboard_web/LICENSE_VISION_UI.md

# 改名 + 删 demo 数据 + 删不要的页面
cd dashboard_web/
# 修改 package.json 的 name 为 arbitrage-dashboard
# 删除 src/views/Dashboard/Billing/ 等不需要的页面
# 修改 src/routes.js 移除多余路由
```

⚠️ **必须保留 Vision UI 的 LICENSE.md 和 README 中的 Creative Tim 版权声明**（MIT 协议要求）。

#### 项目结构（dashboard_web，基于 Vision UI 改造）

```
services/dashboard_web/
├── Dockerfile                        # 多阶段构建：Node builder + nginx 托管
├── package.json                      # 改 name 为 arbitrage-dashboard
├── README.md                         # 项目说明 + 致谢 Vision UI
├── LICENSE_VISION_UI.md              # ⚠️ 必须保留 MIT 原文
├── public/
├── src/
│   ├── assets/                       # Vision UI 自带：字体、图标、图片
│   ├── components/                   # Vision UI 自带：基础组件
│   │   ├── Cards/
│   │   ├── Tables/
│   │   ├── Sidenav/                 # 侧边栏（保留改造）
│   │   └── Configurator/            # 主题配置面板（保留）
│   ├── layouts/                      # 业务页面（按 P1/P2/P3 重写）
│   │   ├── dashboard/                # 概览页（基于 Vision UI 模板改）
│   │   ├── futures/
│   │   │   ├── positions/            # ⭐ 我们写
│   │   │   ├── orders/
│   │   │   ├── trades/
│   │   │   ├── intraday/             # K 线图（TradingView）
│   │   │   ├── kline/
│   │   │   └── statement/
│   │   ├── ai/
│   │   │   ├── scores/
│   │   │   └── history/
│   │   ├── nodes/                    # 节点状态
│   │   ├── host/                     # 主机性能
│   │   ├── strategies/               # 策略管理（Monaco Editor）
│   │   ├── risk/
│   │   ├── audit/
│   │   └── sentiment/                # 舆情看板（P3）
│   ├── examples/                     # Vision UI 自带：示例（参考用，不删）
│   ├── api/                          # ⭐ 我们写：orchestrator API 客户端
│   │   ├── client.ts
│   │   ├── ws.ts
│   │   └── queries/                  # TanStack Query hooks
│   ├── stores/                       # ⭐ 我们写：Zustand 全局状态
│   ├── charts/                       # ⭐ 我们写：图表组件封装
│   │   ├── tremor/                   # Tremor 图表封装
│   │   ├── tradingview/              # TradingView K 线封装
│   │   └── apex/                     # 保留的 ApexCharts（Vision UI 自带）
│   ├── theme/                        # Vision UI 自带主题（颜色已经够用）
│   ├── routes.js                     # 路由配置（按页面清单改）
│   ├── context/                      # Vision UI 自带：UI 状态
│   ├── App.js
│   └── index.js
└── tests/
    ├── e2e/                          # Playwright
    └── unit/                         # Jest（Vision UI 自带）
```

#### Dockerfile（多阶段构建 + nginx）

```dockerfile
# Stage 1: 构建
FROM node:18-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# Stage 2: nginx 托管静态资源
FROM nginx:alpine
COPY --from=builder /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

镜像大小：~50MB（nginx:alpine + 静态资源）

#### 实施工作量预估（v2.3 修订）

因直接用 Vision UI 模板，工作量比 v2.2 自己写少 1-2 周：

| 阶段 | 工作内容 | 工作量 |
|---|---|---|
| **Sprint 0** | Clone Vision UI + 改名 + 装 Tremor + 配 Tailwind | 1 天 |
| **Sprint 1** | API 客户端 + WebSocket + Zustand + 类型定义 | 3 天 |
| **Sprint 2** | 概览页改造（基于 Vision UI 仪表盘 → 接业务数据） | 3 天 |
| **Sprint 3** | 持仓 + 委托 + 成交（基于 Vision UI 表格模板） | 3 天 |
| **Sprint 4** | 节点状态 + 主机监控（基于 Vision UI 状态卡片） | 2 天 |
| **Sprint 5** | AI 评分 + 历史曲线（Tremor） | 2 天 |
| **联调 + 优化** | 与 orchestrator 全链路对接 + 性能调优 | 3 天 |
| **P1 子集小计** | | **约 17 个工作日（3.5 周）** ⬇️ |
| | | |
| Sprint 6 (P2) | K 线图 + 分时图（TradingView Lightweight Charts） | 5 天 |
| Sprint 7 (P2) | 主机详情 + 节点详情 + AI 历史曲线增强 | 3 天 |
| Sprint 8 (P3) | 策略管理（Monaco YAML） + 风控参数 | 5 天 |
| Sprint 9 (P3) | 对账单 + 审计日志 + 决策溯源 + 舆情看板 | 5 天 |
| **P2/P3 小计** | | **约 18 个工作日（3.5 周）** |
| | | |
| **完整版总计** | | **约 35 个工作日（7 周）** ⬇️ |

⭐ **节省时间**：v2.2 估算 48 天 → v2.3 估算 35 天，**省 13 天 (约 2.5 周)**。

#### 实施周期更新

| 阶段 | v2.2 | **v2.3** |
|---|---|---|
| P1 总周期 | 13 周 | **11 周** ⬇️ |
| P1 Dashboard 部分 | 5 周 | **3.5 周** ⬇️ |
| 完整版总周期 | 20 周 | **18 周** ⬇️ |

#### 部署方式（不变）

静态导出 + nginx 托管：
- 镜像 ~50MB
- 内存占用 ~50MB
- 零运维成本

#### 法律 / 合规

- ✅ Vision UI MIT 协议，**完全免费商用**
- ⚠️ 必须保留原 LICENSE.md 和 README.md 中 Creative Tim 的版权声明
- ✅ 可以闭源你自己的修改（个人交易系统不需要开源）

---


## 关键技术选型

| 项 | 选型 | 一句话理由 |
|---|---|---|
| Go 版本 | **1.22** LTS | CTP Go 库测试覆盖最好，2024 年新语义稳定 |
| CTP Go SDK | **openctp/ctp-go** | 最活跃开源 CTP 实现，支持 SimNow 和多家仿真柜台 |
| Python | **3.12** | 性能最优，依赖兼容性好 |
| Python 包管理 | **uv** | 比 pip 快 10-100 倍，原生支持 pyproject.toml |
| **跨服务契约** | **Protobuf + buf** | SSOT 自动生成 Go/Python 代码，杜绝字段不一致 |
| **服务架构风格** | **Hexagonal / Clean Architecture** | domain 零依赖、adapters 隔离 IO，单元测试可脱网跑 |
| **Agent 分层** | **2 层（orchestrator + trading-agent）** | 职责完全分离：UI 协议 vs 交易决策 |
| **orchestrator ↔ trading-agent** | **gRPC** | 强契约 + 高性能 + 流式推送支持 |
| **dashboard ↔ orchestrator** | **HTTP REST + WebSocket** | 浏览器原生支持，HTMX 友好 |
| **Broker 抽象层** | **Go interface + 适配器模式** | V1 仅 CTP 实现，V2 加股票时新增实现，业务零修改 |
| LLM SDK | **直接 OpenAI 兼容 SDK**（不用 litellm） | DeepSeek 提供 OpenAI 兼容 API，二级熔断手写 Gateway 更可控 |
| Redis 模式 | **Streams（事件） + KV（状态）** | Streams 用于 tick/signal 推送，KV 用于 ai_score/状态 |
| **Dashboard 模板** ⭐v2.3 | **Vision UI Dashboard React (MIT 开源)** | Creative Tim 出品，金融科技深色风原型，省 1-2 周纯 UI 工作量 |
| **Dashboard 框架** ⭐v2.3 | **CRA + React 17 + JavaScript**（Vision UI 自带） | 不主动迁移 Next.js / TypeScript，避免兼容性踩坑 |
| **Dashboard 样式** ⭐v2.3 | **Material-UI v5 + Emotion**（Vision UI 自带） | 无需引入 shadcn/ui |
| **Dashboard 图表** ⭐v2.3 | **混合：MUI 原生 + Tremor + TradingView Lightweight Charts + ApexCharts** | K 线用 TradingView；趋势/KPI 用 Tremor；漏斗等保留 ApexCharts |
| **Dashboard 状态** ⭐v2.3 | **TanStack Query + Zustand** | 替换 Vision UI 自带的 Context 直调 |
| **Dashboard 部署** ⭐v2.3 | **静态导出 + nginx** | 最小镜像 ~50MB + 零运维 |
| InfluxDB | **2.7 LTS** | 客户端库稳定，支持到 2026 年底，覆盖项目周期 |
| Dashboard 前端 | **Go html/template + HTMX** | 个人项目无需 React 工具链，零构建步骤 |
| 配置管理 | **Viper（Go）+ python-dotenv** | 两边都从 `.env` 读，Docker 注入环境变量 |
| 容器编排 | **Docker Compose v2** | 单机部署，K8s 杀鸡用牛刀 |

### InfluxDB Bucket 设计

| Bucket | 内容 | 保留期 |
|---|---|---|
| `ma_tick` | 甲醇 Tick 全量（含 5 档盘口） | 30 天 |
| `ma_1min` | 1 分钟 K 线（Tick 聚合） | 90 天 |
| `indicator` | MA/布林带/ATR 等指标值 | 30 天 |
| `ai_score_history` | AI 评分历史（含 shadow 评分） | 30 天 |

---

## 关键代码契约（接口先定，公式后调）

### 契约 SSOT：`contracts/*.proto`

升级到 v1.4 后，**所有跨服务通信契约由 Protobuf 定义**，Go 和 Python 代码**自动生成**，杜绝两边字段不一致。

```protobuf
// contracts/ai_score.proto
syntax = "proto3";
package arbitrage.v1;

message AIScore {
  string  symbol      = 1;   // "MA2509"
  int64   timestamp   = 2;   // unix ms
  int64   expires_at  = 3;   // 默认 +15min
  double  score       = 4;   // [-1, +1] 方向+置信度
  double  confidence  = 5;   // [0, 1] 评分本身可信度
  string  horizon     = 6;   // "intraday"
  string  regime      = 7;   // "trending_up/down/ranging/volatile"
  repeated Driver  key_drivers = 8;
  repeated string  model_chain = 9;
  repeated string  sources     = 10;
  string  raw_text    = 11;
}

message Driver {
  string factor    = 1;   // "甲醇港口库存"
  double weight    = 2;   // 0.4
  string direction = 3;   // "+" 看多 / "-" 看空
}
```

### 代码生成流程

```bash
# 在 contracts/ 目录下：
make gen
# 等价于：
#   protoc --go_out=generated/go/ --python_out=generated/python/ *.proto
#
# 生成产物：
#   contracts/generated/go/aiscore.pb.go    → go-engine 通过 go.mod replace 引用
#   contracts/generated/python/ai_score_pb2.py → python-agent 通过 pyproject 路径引用
```

**字段更新流程**：
1. 修改 `.proto` 文件
2. 跑 `make gen`
3. 两端代码自动得到新 struct
4. 编译期发现的字段不一致直接报错（不会跑到生产才暴雷）

### 工具链选型：buf vs protoc

**推荐 `buf`**（Buf CLI）替代原生 protoc：
- 内置 lint（防止字段编号冲突、命名不规范等常见错误）
- 内置 breaking change 检测（避免破坏向后兼容）
- 配置文件清晰（`buf.yaml` + `buf.gen.yaml`）
- Go/Python/TypeScript 多语言生成统一管理

### 决策融合公式（V1 简化版）

```
finalBias    = score × confidence × decay × alpha
positionSize = technicalSignal.strength × (1 + finalBias)

其中：
  alpha = 0.2  (V1 轻权重，max ±20% 仓位影响)
  decay = 1.0 / 0.5 / 0.0  (按评分老化时间)
```

---

## 服务器初始化与安全加固

> ⚠️ **公网 IP 一旦分配，服务器就暴露在互联网上**。SSH 22 端口每天会收到成百上千次暴力破解尝试。M1 阶段必须先做安全加固，再做其他任何事。

### `scripts/init_server.sh` 一键加固脚本

**M1 阶段产出物**，在阿里云 ECS 首次登录后立即执行（用 `ecs-user` 登录，脚本内部用 `sudo` 提权）。脚本职责：

| 步骤 | 动作 | 目的 |
|---|---|---|
| 1 | 更新系统 + 安装基础工具（curl/git/vim/htop/ufw/fail2ban） | 基础环境 |
| 2 | 把 `ecs-user` 加入 `docker` 组，免 sudo 跑 docker 命令 | 工作账户配置 |
| 3 | 修改 SSH 端口（22 → 自定义如 `22922`） | 减少 99% 的扫描噪音 |
| 4 | `PasswordAuthentication no` + `PermitRootLogin no` | 关闭密码登录和 root 远程登录 |
| 5 | 配置 `fail2ban`，SSH 失败 3 次自动封 IP 1 小时 | 防暴力破解 |
| 6 | 启用 `ufw` 防火墙：默认拒绝入方向，仅放行 自定义SSH端口 + Tailscale 子网（100.64.0.0/10） | 主机层防火墙 |
| 7 | 安装 Docker CE + Docker Compose v2 | 容器运行时 |
| 8 | 安装 Tailscale 并接入 tailnet | 后续 Mac 走内网访问 Dashboard |
| 9 | 设置时区 `Asia/Shanghai` + 启用 NTP 同步 | 交易系统时间必须准确 |
| 10 | 配置 swap（4G）+ 内核参数调优（`vm.swappiness=10`、`net.core.somaxconn=4096`） | 防内存波动、提升网络性能 |
| 11 | 写入欢迎横幅（MOTD）显示项目名 + 风险提示 | 误登录其他机器时一眼识别 |

> 💡 **登录用户决策**：购买 ECS 时**选用 `ecs-user` 作为登录名**（阿里云 2025 年新增的安全推荐选项），不要用 root。理由：日常运维用 `sudo` 即可覆盖所有需要 root 权限的场景；root 直接登录一旦密钥泄露就是全系统沦陷，而 ecs-user 多一道权限墙。
>
> 项目工作账户就是 `ecs-user` 本身，**不再额外创建 trader 等账户**（避免不必要的复杂度）。

执行方式（购买 ECS 时选 `ecs-user` + 配置密钥对，首次 SSH 登录后）：
```bash
ssh -i <你的密钥.pem> ecs-user@<ECS公网IP>
git clone <仓库地址> ~/arbitrage-system
cd ~/arbitrage-system && bash scripts/init_server.sh
# 脚本内部自动 sudo，全程仅需输入一次 sudo 密码（或免密如果配过）
```

### 阿里云安全组配置（云防火墙，比 ufw 更外层）

**入方向规则**（默认拒绝其他全部）：

| 协议 | 端口 | 源 IP | 用途 |
|---|---|---|---|
| TCP | 22922（自定义 SSH） | 你家公网 IP / 0.0.0.0/0 | 远程运维 |
| TCP | 41641（Tailscale UDP） | 0.0.0.0/0 | Tailscale 直连穿透 |
| ICMP | -1 | 0.0.0.0/0 | ping 测试（可选） |

**严禁开放**：6379 (Redis)、8086 (InfluxDB)、8080 (Dashboard) 到公网入方向 — 这些服务**仅通过 Tailscale 内网访问**。

**出方向规则**：全部允许（要连 CTP 前置机、SimNow、DeepSeek/智谱 API、apt 源、Docker Hub）。

### 三道防线总结

```
┌─────────────────────────────────────┐
│ 第一道：阿里云安全组（VPC 层）      │  ← 拦截 99% 的扫描和入侵
├─────────────────────────────────────┤
│ 第二道：ufw 主机防火墙（OS 层）     │  ← 二次防御，防止安全组配错
├─────────────────────────────────────┤
│ 第三道：fail2ban + SSH 密钥（应用层）│  ← 防暴力破解
└─────────────────────────────────────┘
```

### Dashboard 访问策略

- **MVP 阶段**：Dashboard 仅绑定 Tailscale 内网 IP，不开放公网
- 你在任何地方访问 Dashboard：先确保设备已加入 Tailscale → 浏览器访问 `http://ali-trade:8080`
- 手机端：装 Tailscale App，开启后即可访问，**不需要 VPN 翻墙**

---

## 老项目代码迁移清单

### 直接拷贝（轻改即用）

| 源 | 目标 | 说明 |
|---|---|---|
| `daily_stock_analysis/AGENTS.md` | `arbitrage-system/CLAUDE.md` | AI 协作规范，改名+改项目描述 |
| `daily_stock_analysis/.gitignore` | `arbitrage-system/.gitignore` | 追加 Go 和 CTP 相关规则 |

### 参考重写（学模式不抄代码）

| 源 | 参考价值 | 重写方式 |
|---|---|---|
| `daily_stock_analysis/data_provider/base.py` | 策略模式 + fallback | 借鉴写 `python-agent/core/llm_gateway.py` 的二级熔断 |
| `daily_stock_analysis/bot/platforms/base.py` | 多平台抽象 | 简化为只支持钉钉的 watchdog 推送（V1） |
| `daily_stock_analysis/api/v1/router.py` | FastAPI 路由组织 | Go Dashboard 用 `net/http`，借鉴分层思路即可 |
| `daily_stock_analysis/ctp_engine/main.py` | CTP API 调用序列 | **核心参考**：学习 openctp 的 Init/Register/Subscribe 顺序，用 Go 重写 |

### 不要看（坑或无关）

- `templates/` + `static/`（股票界面前端产物）
- `integrations/ai-hedge-fund/` + `vibe-trading/`（第三方子仓库）
- `strategies/*.yaml`（股票策略，期货完全不同）
- `docker/docker-compose.yml`（无 Redis/InfluxDB，架构不匹配）

---

## 五大风险点与缓解

| # | 风险 | 缓解措施 | 先决条件 |
|---|---|---|---|
| 1 | **CTP Go SDK cgo 仅 Linux 可编译** | build tag 隔离 + Docker 内编译 + 本地 mock | ECS 必须 Linux amd64 |
| 2 | **SimNow 凭证审核 1-3 工作日** | M1 阶段就申请，等待期用 mock 数据开发 | **用户立即去 simnow.com.cn 申请** |
| 3 | **DeepSeek API 偶发不稳定** | 二级熔断 + 安全停机，不裸奔 | 用户准备 DeepSeek + 智谱双 API key |
| 4 | **CTP 收盘断线 + 主力合约切换** | 自动重连（指数退避）+ 月度合约切换检测 | 交易日历配置硬编码 |
| 5 | **阿里云 ECS 价格波动** | 购买前实际查询，不要盲信本方案估值 | 用户上控制台核价后再购买 |

---

## 接下来的第一件事（立即可执行）

**今天/本周内，并行启动 3 件不依赖技术细节的事**：

1. **去 [simnow.com.cn](http://simnow.com.cn) 注册 SimNow 模拟账户**（拿到 BrokerID/UserID/Password，1-3 工作日）
2. **登录阿里云控制台**，按上海地域 + 4C8G u1 + 1 年期实际查价并下单（包年优惠通常 30-50%，请优先选 u1 系列**避开突发型 t6/e**）
3. **申请 DeepSeek API key**（[platform.deepseek.com](http://platform.deepseek.com)，预存 50 元够用 1-2 个月）+ **申请智谱 GLM-4 API key**（备用）

⚠️ **代码托管平台还没确认**（GitHub/Gitee/自托管），可以等到 M1 末尾再决定，**不阻塞前置准备**。

---

## 验证方式（E2E Smoke Test，10 步全部 SimNow 不碰真钱）

1. **基础连通性**：`ping ali-trade` + `redis-cli ping` + `curl :8086/ping`
2. **CTP 行情接入**：日志含 `OnFrontConnected` + InfluxDB Tick count > 0
3. **AI 评分写入**：`redis-cli get ai_score:MA2509:live` 返回完整 JSON
4. **策略信号生成**：日志含 `SIGNAL: BUY/SELL/HOLD`
5. **模拟报单**：日志含 `OnRtnOrder` + `OnRtnTrade`
6. **风控拦截**：极端参数下日志含 `RISK_BLOCK`
7. **Dashboard 展示**：浏览器看到完整盘面 + 持仓 + AI 评分
8. **一键平仓**：点击按钮 → SimNow 收到平仓委托
9. **AI 评分老化**：停 Python Agent 31 分钟 → Go 引擎切换"AI=0 模式" + 钉钉告警
10. **断线重连**：手动 kill 容器 → 1 秒内自动重启 + 状态从 Redis 恢复

---

## 关键文件速查（开发时最常打开的 7 个）

- `contracts/orchestrator.proto` — orchestrator gRPC 服务定义（编排层契约）
- `contracts/ai_score.proto` — ⭐ AI 评分契约 SSOT（修改这里两端自动更新）
- `services/trading_agent/internal/adapters/broker/interface.go` — Broker 抽象（V2 加股票从这里扩展）
- `services/trading_agent/internal/domain/decision/fuser.go` — 决策融合公式实现
- `services/trading_agent/internal/api/grpc/server.go` — trading-agent 暴露给 orchestrator 的 gRPC 服务
- `services/orchestrator_agent/internal/api/rest/handlers.go` — orchestrator 暴露给 dashboard 的 REST API
- `services/llm_scorer/adapters/llm_gateway.py` — 二级熔断网关

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-12 | 初稿，定型核心架构（全部上云 + 二级 LLM 熔断 + AI 轻权重 + 钉钉告警） |
| v1.1 | 2026-06-13 | 新增「服务器初始化与安全加固」章节 + `scripts/init_server.sh` 一键脚本规划；明确公网 IP 必选 + 3M 固定带宽（约 ¥600/年）；M1 交付物增加安全加固验证项 |
| v1.2 | 2026-06-13 | 登录用户从 root 改为 `ecs-user`（阿里云购买页推荐选项）；`init_server.sh` 移除"创建非 root 账户"步骤，改为"将 ecs-user 加入 docker 组"；明确不再额外创建 trader 等账户 |
| v1.3 | 2026-06-13 | 数据盘改为延后购买策略（M1 阶段先 40GB 系统盘，触发线后再加挂载）；实例规格说明扩充到 u 系列（u1/u2a/u2i 任选）；采购清单增加可用区选择、登录凭证、镜像版本等细节；Watchdog 职责增加系统盘使用率监控 |
| v1.4 | 2026-06-13 | ⭐ **微服务架构升级**：新增「架构原则」章节（5 条硬规则 + monorepo 决策 + 通信协议分层 + Broker 抽象层）；目录结构升级为 Hexagonal Architecture（domain/adapters 分层）；契约从手写 Go struct + Pydantic 升级为 Protobuf + buf 自动生成；新增顶级 `contracts/` 目录；docker-compose.yml 增加 profiles 支持模块化启动；M1 交付物增加 contracts/ 工具链就绪 |
| **v1.5** | **2026-06-13** | ⭐⭐ **两层 Agent 架构升级**：原 `go-engine` 拆分为 4 个微服务（`ctp_tick` + `indicator` + `trading_agent` + `orchestrator_agent`）；原 `python-agent` 改名 `llm_scorer`；新增 `orchestrator-agent` 作为"面向人类的代理"（UI 路由 + 状态聚合 + 指令转发）；trading-agent 作为"面向机器的代理"（决策 + 风控 + 报单）；两层 agent 间走 gRPC；dashboard 完全解耦不再读 Redis；新增详细架构图章节；M3/M4/M5 里程碑重新拆分；服务统一放 `services/` 顶级目录 |
| **v1.6** | **2026-06-13** | 新增「服务发现与启动协调」章节：明确不引入 Consul/Etcd，用 Docker Compose DNS + 静态环境变量 + Redis KV 轻量注册；规定客户端必须自动重连；规定启动顺序双保险（healthcheck + 重试）；服务自报家门到 Redis（30s TTL）让 orchestrator 知道在线服务列表 |
| **v1.7** | **2026-06-13** | ⭐⭐⭐ **业务能力大升级**：(1) 新增 `db_writer` 服务实现写入串行化（CQRS 写侧 + Redis Stream 队列）；(2) 新增 `sentiment_monitor` 服务做舆情监控（V1 财联社 RSS 起步，V3 扩展多源）；(3) 新增「策略与指标的 WebUI 管理」机制（YAML 配置 + 交易时段守卫 + 灰度发布 shadow→live）；(4) 新增「数据可靠性」机制（Redis AOF 持久化 + 关键数据双写 InfluxDB + trading-agent 三步启动对账，CTP 查询为黄金真相）；(5) 新增「事件驱动 AI 介入」机制（剧烈波动触发 LLM 紧急分析 + 谨慎模式行为）；(6) 新增 Dashboard 页面规划章节（V1/V2/V3 渐进 16 个页面）；(7) 里程碑从 8 周 6 个 M 重组为 16 周 P1/P2/P3 三阶段（P1 核心 8 周、P2 稳定 4 周、P3 完整 4 周）；(8) 总服务数从 8 个增至 10 个 |
| **v1.8** | **2026-06-13** | 重新核算 10 个服务内存占用（P1 ≈ 6.6GB / P3 ≈ 8.9GB）；新增「资源预算与降级策略」章节；固化决策：先 4C8G 起步，P2 末期评估升 8C16G；InfluxDB 缓存调小到 512MB、Python 服务 mem_limit 强制限制；P1 阶段 sentiment_monitor 限定为财联社单源 5 分钟一次；Watchdog 内存告警三级（80%/90%/95%）；月成本最贵 ¥930 仍在 1500 预算内 |
| **v1.9** | **2026-06-13** | ⭐⭐ **舆情系统三层重构**：原 `sentiment_monitor` 单服务拆分为三个独立服务 `domestic_intel`（国内源 P1）/ `overseas_intel`（海外源 P3）/ `social_intel`（Telegram P4）；新增「出海方案」决策（阿里云香港跳板 ECS 1C1G ~¥100-150/月）；明确不做 Discord（中文化工圈使用率低）和 X/Twitter（API $200/月起步）；新增「RSS 动态新增」UI 功能（M8 阶段）；Telegram 群组监听放 V4 后续；服务总数从 10 个增至 12 个；P3 月成本含香港代理 ¥930-1080，仍在 1500 预算内 |
| **v2.0** | **2026-06-13** | ⭐⭐⭐ **数据可追溯性大版本**：新增「机制 6：数据清洗管道与可追溯性」章节，定义 5 阶段清洗流水线（Fetch/Parse/Clean/Understand/Aggregate）+ 6 层 ID 血缘体系（raw → parsed → cleaned → analysis → sentiment_snapshot → ai_score → decision → order）；明确 LLM 用与不用原则（LLM 只做语义理解，其他用规则）；定义 7 个 InfluxDB bucket schema 永久保留作审计资产；时间倒带回测设计（禁止重新调 LLM，必须用历史 LLM 输出）；P3 阶段做决策溯源 Dashboard 页面；存储成本预估 1.8GB/年（V1）→ 42GB/年（P3 多合约）；LLM 调用成本仅 ¥3-18/月 |
| **v2.1** | **2026-06-13** | ⭐ **服务精简合并**：v2.0 的 12 个微服务合并到 8 个：(1) `indicator` → 并入 `trading_agent` 的 `domain/indicator/` 包（生命周期完全绑定 + 减少 Redis 跳数 + 降低延迟）；(2) `dashboard_web` → 并入 `orchestrator` 的 `api/views/` + `static/`（同为 Go 模板渲染，无分离意义）；(3) `domestic_intel` + `overseas_intel` + `social_intel` → 合并为单一 `intel_collector`（配置驱动，YAML 指定数据源 + 是否走代理）；服务总数 12 → 8（-33%），配置项 -33%，启动时间 -50%，重复代码 -66%；docker-compose profiles 同步精简（dashboard → ui） |
| **v2.2** | **2026-06-13** | ⭐⭐ **Dashboard 技术栈大升级**：放弃 HTMX，升级为 **Next.js 14 + TypeScript + TailwindCSS + shadcn/ui + Tremor + TradingView Lightweight Charts** 全栈；引入金融科技深色风设计（参考用户提供的设计原型）；定义完整设计令牌（颜色 / 字体 / 效果）；dashboard_web 从 orchestrator 重新拆出独立服务（异构语言无法合并），服务数 8 → 9；用户选择 P1 就上高级版，P1 周期从 8 周延长到 13 周（含 5 周 Dashboard 开发），M5 拆分为 M5 (orchestrator+watchdog) + M6 (Dashboard 高级版 5 周)；总周期 16 周 → 20 周；推荐降级路径（买商业模板省 1-2 周） |
| **v2.3** | **2026-06-13** | ⭐⭐ **Dashboard 改用 Vision UI 开源模板**：放弃从零搭建 Next.js + shadcn/ui 方案，**直接 fork Creative Tim 的 Vision UI Dashboard React (MIT 开源)** 作为起点；技术栈调整为 CRA + React 17 + JavaScript + Material-UI v5（Vision UI 自带，不主动迁移）；图表混合方案：K 线用 TradingView Lightweight Charts、趋势/KPI 用 Tremor、漏斗等保留 Vision UI 自带 ApexCharts、容器用 MUI 保持视觉统一；Dashboard 工作量从 5 周降到 3 周，P1 周期从 13 周降到 11 周，总周期从 20 周降到 18 周；删除 v2.2 重复的 Dashboard 章节（Next.js 设计令牌、shadcn/ui 配置等不再适用） |
