# 工程规则与代码约束（Engineering Guide）

> **本文档是项目的"宪法"。任何在此项目上工作的人或 AI agent，**写代码前必须读这一份**。**
> 所有违反这些规则的 PR / commit / 修改，**一律拒绝合入**。

---

## 0. 阅读顺序（任何人/agent 第一次接手必读）

```
1. ENGINEERING_GUIDE.md（本文档） — 工程规则
2. docs/ARCHITECTURE.md           — 架构方案（v2.3，约 2600 行）
3. contracts/README.md             — 跨服务契约管理
4. services/<你要改的模块>/README.md — 该模块详细文档
```

读完上述 4 个文档**之后**再开始写代码。

---

## 1. 项目定位

- **是什么**：AI 辅助日内趋势交易系统（个人交易者用，主交易**甲醇 MA 期货**）
- **不是什么**：高频做市系统、量化基金平台、SaaS 多租户产品
- **核心价值**：稳定 > 智能 > 美观；**不亏钱 > 赚多钱**

⚠️ **写代码时永远问自己**：「这个改动会让账户更安全，还是更危险？」

---

## 2. 七条不可妥协的硬规则（来自 ARCHITECTURE.md）

### 规则 1：每个服务独立进程、独立 Docker 镜像、独立部署
- 一个微服务 = 一个 Docker 容器 = 一份独立 Dockerfile
- 服务间**绝不共享内存**（不允许进程内函数调用替代网络调用）
- 每个服务能**单独 `docker compose --profile <name> up`** 启动

### 规则 2：服务间只通过 `contracts/` 定义的协议通信
- **唯一真相源（SSOT）**：`contracts/*.proto`
- Go/Python 代码**由 protoc/buf 自动生成**，禁止手写跨服务结构体
- 禁止通过 Redis 写非 contract 定义的字段（如临时加 `debug_xx` 绕过协议）
- 改契约流程：改 `.proto` → `make gen` → 两端代码同步 → CI 验证

### 规则 3：业务逻辑（domain）零外部依赖，IO 全在 adapters
- 每个服务分两层：`domain/`（纯业务） + `adapters/`（外部 IO）
- `domain/` **禁止 import**：Redis 客户端、InfluxDB 客户端、CTP、HTTP 库、文件 IO
- `domain/` 通过 interface（Go）/ ABC（Python）声明所需能力，由 `adapters/` 实现
- 这是 **Hexagonal Architecture**，让单元测试**毫秒级 + 脱网**

### 规则 4：每个服务必须有 `/healthz` 和 `/metrics` 端点
- `/healthz`：返回服务是否健康（依赖检查 + 业务关键指标）
- `/metrics`：Prometheus 格式
- Watchdog 周期巡检 healthz，不健康即推钉钉
- 端点定义见 `contracts/healthz.proto`

### 规则 5：单元测试不依赖外部服务
- `tests/unit/` 必须能在**无 Redis、无 InfluxDB、无网络**的环境跑通
- 外部依赖一律用 mock / fake（`adapters/` 的 interface 让这件事简单）
- `tests/integration/` 才能用真服务（CI nightly 跑）
- **每个 PR 必跑 unit 测试**，integration 测试只在 nightly

### 规则 6：代码中禁止魔法数字（Magic Numbers）
- 除 `0`、`1`、`-1` 外，所有数字字面量**必须**定义为具名常量
- 配置值（超时时间、阈值、权重等）必须定义为常量或从环境变量读取，**禁止**直接在逻辑中写裸数字
- 即使是"一次性"使用的数字也要命名——名字本身就是文档

```go
// ❌ 禁止
if ageSeconds > 900 {
    return 0.5
}
time.Sleep(30 * time.Second)

// ✅ 正确
const ScoreStaleThresholdSec = 900
const CTPSessionStartDelay = 30 * time.Second

if ageSeconds > ScoreStaleThresholdSec {
    return ScoreDegradedWeight
}
time.Sleep(CTPSessionStartDelay)
```

```python
# ❌ 禁止
if price_change > 0.01:
    trigger_emergency()

# ✅ 正确
PRICE_SPIKE_THRESHOLD = 0.01  # 1% 单分钟涨跌幅

if price_change > PRICE_SPIKE_THRESHOLD:
    trigger_emergency()
```

这一条是对**架构中所有硬编码数值**（AI 评分老化 15min/30min、alpha=0.2、止损 0.5%、紧急触发 1%、防抖 5 分钟 等）的代码级约束。

### 规则 7：Go 服务 Redis 连接统一初始化（pkg/redis/）
- 所有 Go 服务**必须**通过 `pkg/redis/` 包操作 Redis，**禁止**各服务独立创建 `*redis.Client`
- `pkg/redis.NewAdapter(addr)` 是唯一的 Redis 连接创建入口
- 各服务的 `adapters/redis/` 只保留**业务逻辑**（如 Tick 双写、消费组订阅），Redis 连接本身通过依赖注入获取 `pkg/redis.RedisClient` 接口
- 单元测试使用 `pkg/redis.NewMockClient()` 替代真 Redis

```go
// ✅ 正确：通过 pkg/redis 统一创建
import pkgredis "github.com/arbitrage-system/pkg/redis"
client := pkgredis.NewAdapter("redis:6379")
writer := redis.NewTickWriter(client)  // 注入接口

// ❌ 禁止：各服务直接创建 go-redis 客户端
import "github.com/redis/go-redis/v9"
rdb := redis.NewClient(&redis.Options{...})  // 不要这样
```

---

## 3. 目录结构约定

### 项目根目录（不可随意新增顶级目录）

```
arbitrage-system/
├── ENGINEERING_GUIDE.md  ← 本文档
├── README.md              ← 项目简介 + 快速开始
├── CLAUDE.md              ← AI 协作规范
├── .gitignore
├── docker-compose.yml     ← 多服务编排（含 profiles）
├── docker-compose.test.yml
├── .env.example
├── Makefile               ← 顶层快捷命令
├── contracts/             ← Protobuf 契约 SSOT
├── pkg/                   ← Go 共享包（如 pkg/redis/），Python 服务不适用
├── services/              ← 9 个微服务
├── scripts/               ← 运维脚本
└── docs/                  ← 架构文档
```

⚠️ **`pkg/` 仅限 Go 共享基础设施**（如 `pkg/redis/` 统一 Redis 连接），Python 服务不适用此目录。禁止往 `pkg/` 放通用工具函数或业务逻辑——业务代码归属 `services/`。

### 单个服务目录约定（每个 services/ 下的子目录）

#### Go 服务（trading_agent / orchestrator / ctp_tick / db_writer / watchdog 等）

```
services/<name>/
├── Dockerfile             ← 多阶段构建
├── go.mod / go.sum
├── README.md              ← 本服务详细说明
├── .env.example           ← 本服务环境变量
├── cmd/main.go            ← 入口（仅初始化 + 装配，不写业务）
├── internal/
│   ├── domain/            ← 业务逻辑（零外部依赖）
│   ├── adapters/          ← 外部 IO 适配
│   ├── api/               ← 对外暴露的端点（healthz/metrics/grpc/rest）
│   └── app/wire.go        ← 装配层（DI）
└── tests/
    ├── unit/              ← 不依赖外部服务
    └── integration/       ← 用真 Redis/InfluxDB
```

#### Python 服务（intel_collector / llm_scorer）

```
services/<name>/
├── Dockerfile             ← python:3.12-slim
├── pyproject.toml         ← uv 管理依赖
├── README.md
├── .env.example
├── main.py                ← 入口
├── domain/                ← 业务逻辑（零外部依赖）
├── adapters/              ← 外部 IO（LLM 客户端 / 数据爬取 / Redis）
├── api/                   ← FastAPI /healthz /metrics
├── config/                ← YAML 配置（如适用）
└── tests/{unit,integration}/
```

⚠️ **新增任何顶级目录到服务下都需要在该服务的 README 中说明，并更新本文档**。

---

## 4. 命名约定

### 文件 / 目录命名

| 类型 | 约定 | 例子 |
|---|---|---|
| 服务名 | snake_case | `trading_agent` / `intel_collector` |
| Go 包名 | 单词小写 | `package indicator` |
| Go 文件 | snake_case.go | `ma_trend.go` / `risk_manager.go` |
| Python 模块 | snake_case.py | `llm_gateway.py` / `redis_writer.py` |
| 配置文件 | snake_case.yaml | `domestic_sources.yaml` |
| 文档 | UPPER_SNAKE.md | `README.md` / `ARCHITECTURE.md` |
| 脚本 | snake_case.sh | `init_server.sh` / `smoke_test.sh` |

### 代码符号命名

| 语言 | 类型 | 约定 | 例子 |
|---|---|---|---|
| Go | 函数（导出） | PascalCase | `func ProcessTick()` |
| Go | 函数（内部） | camelCase | `func processTick()` |
| Go | 接口 | PascalCase + 后缀 -er | `type Broker interface` |
| Go | 结构体 | PascalCase | `type AIScore struct` |
| Go | 常量 | PascalCase | `const MaxRetries = 3` |
| Python | 函数 / 变量 | snake_case | `def fetch_news()` |
| Python | 类 | PascalCase | `class LLMGateway` |
| Python | 常量 | UPPER_SNAKE | `MAX_RETRIES = 3` |
| 全局 | Redis Key | 冒号分隔 | `ai_score:MA2509:live` |
| 全局 | Redis Stream | 冒号分隔 | `tick:MA2509` / `write:tick` |
| 全局 | 环境变量 | UPPER_SNAKE | `REDIS_ADDR` / `CTP_BROKER_ID` |

### Redis Key 命名规范（必须遵守）

```
<domain>:<symbol>:<variant>     一般状态
ai_score:MA2509:live
ai_score:MA2509:shadow
ai_score:MA2509:emergency
sentiment:MA2509:current
position:current
indicator:MA2509:boll_20

write:<bucket>                  写入流（→ db_writer 消费）
tick:<symbol>                   行情流
news_stream                     舆情流
signal:<symbol>                 信号流

services:<name>                 服务注册（30s TTL）
strategies:active               活跃策略列表
```

⚠️ **新增 Redis Key 必须在所属服务的 README 中登记**。

---

## 5. 通信协议规则

### 服务间通信只走以下 4 种渠道

| 协议 | 场景 | 何时用 |
|---|---|---|
| **Redis Streams** | 事件流（Tick / 信号 / 舆情） | 异步、有序、可重放 |
| **Redis KV + TTL** | 共享状态（评分 / 持仓快照） | 读多写少、需要老化 |
| **gRPC** | orchestrator ↔ trading_agent | 强契约 + 高性能 |
| **HTTP REST + WebSocket** | dashboard ↔ orchestrator | 浏览器友好 |

⚠️ **禁止**直接的服务间 HTTP 调用（如 trading_agent 直接调 llm_scorer），都要走 Redis 或 gRPC。

### 唯一例外：watchdog → 各服务 `/healthz`
监控者主动巡检健康端点是允许的（HTTP GET）。

---

## 6. 配置管理规则

### 环境变量

每个服务必须有 `.env.example`，列出所有需要的环境变量 + 默认值 + 说明。

⚠️ **绝对禁止**：
- 把 `.env` 入库（已在 `.gitignore` 里）
- 在代码里硬编码 API key / 密码
- 写共用配置文件给多个服务读（每服务独立 `.env`）

### 服务发现

- 服务地址走 **Docker DNS**（容器名即 hostname）
- 不引入 Consul / Etcd / Nacos 等注册中心
- 服务自报家门到 Redis：`SET services:<name> <info> EX 30`

### 配置变更

| 配置类型 | 修改后 | 原因 |
|---|---|---|
| `.env.example` | 同步本服务 README | 让别人知道有这个变量 |
| 业务配置（如交易日历） | 在代码里硬编码，每年初手动更新 | 简单可靠，无运行时配置中心 |
| YAML 数据源配置（intel_collector） | 通过 WebUI（M8）或直接改文件 | 配置驱动 |

---

## 7. 错误处理与可观测性

### 错误处理（Go）

```go
// ✅ 好的做法：错误一直往上传，附带上下文
if err != nil {
    return fmt.Errorf("订阅 CTP 行情失败: symbol=%s: %w", symbol, err)
}

// ❌ 禁止：吞掉错误
if err != nil {
    return nil  // 永远不要这样
}

// ❌ 禁止：泛泛的错误信息
return errors.New("error")  // 没有上下文，调试痛苦
```

### 错误处理（Python）

```python
# ✅ 好的做法
try:
    result = llm.analyze(news)
except LLMTimeoutError as e:
    logger.error(f"LLM 超时 prompt_len={len(prompt)}: {e}")
    raise

# ❌ 禁止：用 except: 吞所有异常
try:
    ...
except:                          # 永远别这样
    pass
```

### 日志规范

| 级别 | 用法 | 例子 |
|---|---|---|
| `DEBUG` | 详细的执行轨迹（开发用） | `tick parsed: symbol=MA2509 price=2350` |
| `INFO` | 关键业务事件 | `订阅 MA2509 成功` / `策略 ma_cross 启用` |
| `WARN` | 异常但可继续 | `Redis 连接抖动，重试中` / `LLM 超时切到 GLM` |
| `ERROR` | 错误，影响功能 | `CTP 报单被拒` / `数据库写入失败` |
| `FATAL` | 必须停机 | `两个 LLM API key 都失效` |

⚠️ **生产环境默认 INFO 级别**，DEBUG 仅本地开发用。

### 日志格式（统一）

```
2026-06-13T10:30:15.123Z  INFO  trading_agent  策略 ma_cross 触发买入信号 symbol=MA2509 strength=0.6 ai_score=0.62
└─时间戳ISO8601─┘ └级别┘ └─服务名─┘ └─人类消息─┘                  └────────结构化字段─────────┘
```

---

## 8. 测试规则

### 单元测试（必须）

- **覆盖率目标**：domain/ 层 ≥ 70%（只测纯逻辑）
- **运行时间**：单服务全量 unit 测试 ≤ 30 秒
- **运行环境**：`go test ./...` / `pytest tests/unit/` 一行跑通，无外部依赖

### 集成测试（建议）

- 跑在 docker-compose.test.yml 起的真服务上
- 仅在 CI nightly 跑（不阻塞 PR）

### 手动冒烟测试

每个里程碑结束跑一次 `scripts/smoke_test.sh`（10 步）。

---

## 9. Git / 提交规范

### 分支策略

- `main`：稳定版本，永远可部署
- `dev`：开发主线
- `feature/<服务名>-<简述>`：单服务新功能
- `fix/<服务名>-<简述>`：单服务 bug 修复
- `chore/<简述>`：非业务改动（依赖升级 / 文档等）

### 分支工作流规则（不可妥协）

**任何新模块、新功能的开发，必须先切分支，禁止直接在 `main` 或 `dev` 上提交。**

```
1. git checkout -b feature/<服务名>-<简述>    ← 从 dev 切出功能分支
2. 在分支上开发 + 提交
3. make check                                ← 本地验证：lint + vet + test 全部通过
4. git push origin <branch>
5. 创建 PR → 合并到 dev
6. dev 验证通过后 → 合并到 main
```

**合并前必须满足**：
- `make check` 全部通过（go vet + golangci-lint + ruff + go test + pytest）
- PR 上 GitHub Actions CI 全绿
- 代码 review（如有人协助）

**紧急修复（hotfix）例外**：
- 可以从 `main` 切 `fix/<简述>` 分支
- 修复后合并回 `main` **和** `dev`
- 仍需通过 `make check` + CI

```bash
# 正确流程 ✅
git checkout -b feature/ctp_tick-add-symbol-filter
# ... 开发 ...
make check
git add -A && git commit -m "feat(ctp_tick): ..."
git push origin feature/ctp_tick-add-symbol-filter

# 错误 ❌
git checkout main
# ... 直接改代码 ...
git commit -m "fix something"  # 不允许！
```

### Commit Message（Conventional Commits）

```
<type>(<scope>): <subject>

<body>

<footer>
```

| type | 用法 |
|---|---|
| `feat` | 新功能 |
| `fix` | bug 修复 |
| `docs` | 文档 |
| `refactor` | 重构（不改行为） |
| `perf` | 性能优化 |
| `test` | 测试 |
| `chore` | 依赖、配置、CI |

| scope | 范围 |
|---|---|
| `trading_agent` / `orchestrator` / ... | 单服务 |
| `contracts` | 契约改动 |
| `docker` | 编排 |
| `docs` | 文档 |

例子：
```
feat(trading_agent): 新增 SafeMode 启动对账机制

- 启动时 Redis → InfluxDB → CTP 三步对账
- 不一致时切 SafeMode（只平仓不开新仓）
- 推钉钉告警

关联 ARCHITECTURE.md 机制 3
```

### 提交前必跑

```bash
make lint              # 各服务 lint
make test-unit         # 各服务 unit 测试
make gen               # 如改了 contracts，重新生成
```

---

## 10. AI Agent 工作准则

> 所有协助本项目的 AI agent（Claude / Cursor / GPT 等）必须遵守。

### 准则 1：先读再写
- 任何修改前先读相关服务的 README + ARCHITECTURE.md 对应章节
- 不要凭"印象"或"通用经验"写代码——本项目有自己的约定

### 准则 2：跨服务改动 = 改契约
- 如果发现两个服务都要改，**先问**：是不是契约要更新？
- 改契约必须改 `contracts/*.proto`，不可绕过

### 准则 3：domain/ 不能 import 任何 IO
- 写代码前确认这条
- 如果 `domain/` 里 import 了 redis/http/io 包 → 立即停下，挪到 adapters/

### 准则 4：不要过度设计
- 这是个人项目，不是 SaaS 平台
- 不要加：注册中心、配置中心、Service Mesh、复杂 ORM
- 加任何依赖前先问：「这是 ARCHITECTURE.md 里有的吗？没有就不要加。」

### 准则 5：不要修改 ARCHITECTURE.md 里的硬决策
本项目以下决策**不可在代码里偷偷改**（必须先在文档里讨论）：

- 部署形态（全云 + Docker）
- 服务划分（9 个）
- 通信协议（Redis Streams + gRPC + REST）
- 仓库形态（Monorepo）
- AI 评分老化机制（< 15min/15-30min/>30min decay 1.0/0.5/0.0）
- LLM 熔断策略（DeepSeek → GLM → 停机告警）
- AI 影响仓位上限（±20%，alpha=0.2）

如果你（agent）认为某条决策需要改，**必须先告诉用户，让用户更新 ARCHITECTURE.md，再改代码**。

### 准则 6：测试优先
- 写完 domain 层的纯逻辑代码必须有 unit 测试
- 测试不通过 = 代码还没写完
- 不要为了"先跑通"而跳过测试

### 准则 7：保留可追溯性（v2.0 机制 6）
- 任何业务数据必须能反查到源头
- 6 层 ID 血缘：raw → parsed → cleaned → analysis → snapshot → score → decision → order
- 这条不可破坏

### 准则 8：报告事实而非揣测
- 测试失败就说失败，不要回避
- 没改完就说没改完
- 跑了什么、跑通什么、什么没跑都明确报告

---

## 11. 性能 / 资源约束

### 内存预算（4C8G 服务器）

| 类别 | 上限 | 监控 |
|---|---|---|
| 单个 Go 服务 | < 600 MB | Watchdog 推钉钉 |
| 单个 Python 服务 | < 1 GB（docker mem_limit 强制） | Docker OOM Killer |
| InfluxDB | < 2.5 GB（cache 限制 512MB） | INFLUXD_STORAGE_CACHE_MAX_MEMORY_SIZE |
| Redis | < 1.2 GB（maxmemory 1G） | allkeys-lru 淘汰 |
| 总体内存使用 | < 80% 触发告警，> 95% 推紧急告警 | 三级告警机制（v1.8） |

### 延迟预算

| 路径 | 上限 |
|---|---|
| Tick → 入 Redis Stream | < 5ms |
| Tick → 指标计算完成 | < 50ms |
| 信号生成 → 报单发出 | < 100ms |
| LLM 评分（一轮） | < 60s |
| LLM 紧急分析 | < 30s |
| Dashboard WebSocket 推送 | < 1s 延迟 |

⚠️ 超出预算就是 bug，必须排查。

---

## 12. 安全规则

### 密钥管理

- API key（DeepSeek/GLM/CTP/钉钉等）只放 `.env`，不入库
- 服务器登录用密钥对（M1 阶段 init_server.sh 强制）
- SSH 端口改非 22（22922 等）+ fail2ban

### 公网暴露

- 默认全部服务**仅监听内网 / Tailscale**
- Dashboard 的 8000 端口走 Tailscale
- 阿里云安全组白名单：仅放行 SSH 自定义端口 + Tailscale 41641
- 严禁开放 6379 / 8086 / 9090 到公网

### 交易安全

- 紧急平仓按钮必须**二次确认**
- 修改型操作（策略/风控）走**交易时段守卫**
- 所有报单进 InfluxDB `orders` bucket 永久保存（审计）
- CTP 查询接口是**黄金真相**，本地状态只是缓存

---

## 13. 文档规则

### 每个服务必须有的文档

```
services/<name>/README.md   ← 必须存在
```

包含：
1. 服务定位（一句话）
2. 职责清单（做什么、不做什么）
3. 内部结构（domain/adapters/api 各负责什么）
4. 对外接口（暴露端口、Redis Key、gRPC 方法、Stream 名）
5. 依赖（Redis / InfluxDB / 其他服务 / 外部 API）
6. 配置（关键环境变量）
7. 启动方式
8. 测试方式
9. 已知限制 / 风险点

### 文档更新规则

| 改动类型 | 必须同步更新 |
|---|---|
| 改契约（.proto） | `contracts/README.md` + 受影响服务的 README |
| 新增服务 | `ARCHITECTURE.md` + 该服务 README + 本文档 |
| 改环境变量 | `.env.example` + 该服务 README |
| 改 Redis Key | 该服务 README + 消费方服务 README |
| 改架构决策 | `ARCHITECTURE.md` 升级版本号 + 变更记录 |

⚠️ **代码改了但文档没改 = PR 不合入**。

---

## 14. 紧急情况处理

### 生产事故 SOP

```
1. 不要 panic（人也不要 panic）
2. 看 Watchdog 钉钉告警 → 定位问题服务
3. 紧急平仓（如需要）
4. 看日志（docker logs <service>）
5. 重启服务（docker compose restart <service>）
6. 不行就 SafeMode（trading_agent 切只平仓不开新仓）
7. 还不行就停机：docker compose --profile futures down
```

### 数据丢失场景

| 场景 | 应对 |
|---|---|
| Redis 重启 | AOF 自动恢复（每秒 fsync） |
| Redis 数据完全丢失 | trading_agent 启动从 InfluxDB 补齐 |
| InfluxDB 重启 | 容器自动重启，无数据丢失 |
| InfluxDB 损坏 | 还原最近备份（建议每周快照） |
| 服务器全死 | 切 SafeMode + 联系期货公司手动平仓 |

---

## 15. 这份文档怎么更新

- 任何修改本文档的 PR 必须经用户审核
- 修改后在底部更新版本号 + 变更记录
- 通知所有协助开发的 agent："规则改了，重新读一遍"

---

## 16. CI/CD 工作流（v1.1 新增）

### 三层检测体系

| 层级 | 触发时机 | 机制 |
|---|---|---|
| **Pre-commit Hook** | 本地 `git commit` 前 | `.githooks/pre-commit` — go vet + golangci-lint + ruff + go test |
| **GitHub Actions** | push/PR 到 main/dev | `.github/workflows/ci.yml` — 矩阵并行检测所有服务 |
| **手动命令** | 任意时间 | `make check` — 一键执行所有检测 |

### 提交前必跑（Pre-commit Hook）

安装：`make install-hooks`（首次 clone 后执行一次）

每次 `git commit` 自动执行：
1. `go vet ./...` — 编译检查（所有 Go 服务）
2. `golangci-lint run ./...` — Go lint（如已安装）
3. `ruff check .` — Python lint（如已安装）
4. `go test ./... -short -count=1` — 单元测试
5. contracts/.proto 变更提醒（如修改了 proto 文件）

任一失败 → 拒绝 commit。跳过（紧急情况）：`git commit --no-verify`

### Makefile 命令

```bash
make check        # ⭐ 一键执行所有提交前检查（等同于 pre-commit hook）
make fix          # 自动修复（gofmt + ruff --fix）
make lint         # 所有语言 lint
make lint-go      # golangci-lint
make lint-python  # ruff
make lint-proto   # buf lint
make vet          # go vet 编译检查
make test-unit    # 所有 unit 测试
make test-int     # 集成测试（需要 Docker）
```

### GitHub Actions CI（`.github/workflows/ci.yml`）

触发条件：push / PR 到 `main` 或 `dev`

| Job | 检测内容 |
|---|---|
| `go-lint-test` | 5 个 Go 服务矩阵并行：go vet + golangci-lint + go test |
| `python-lint-test` | 2 个 Python 服务：ruff + pytest |
| `proto-check` | buf lint + breaking change 检测 |
| `frontend-check` | dashboard_web：lint + build check |
| `ci-summary` | 汇总（全部 Job 通过才绿） |

### CI 通过标准

- 所有 Go 服务 `go vet` 通过
- `golangci-lint` 零错误
- `ruff check` 零错误
- 所有 unit 测试通过
- `buf lint` 通过
- dashboard_web build 成功

### 本地环境准备

```bash
# Go lint 工具
brew install golangci-lint

# Python lint 工具
pip install ruff

# 安装 git hook
make install-hooks
```

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版，固化 ARCHITECTURE.md v2.3 中的所有工程约束 |
| v1.1 | 2026-06-13 | 新增 CI/CD 工作流章节（三层检测体系 + pre-commit hook + GitHub Actions） |
