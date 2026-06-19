# ARCHITECTURE_GUARDRAILS.md — 正式数据流与禁止 Mock 业务化约束

> 本文档记录 `arbitrage-system` 的架构红线。
>
> 目标：防止业务代码为了测试方便内置 mock/stub 数据、写死关注合约、绕过正式服务链路。所有后续 agent 在实现新功能、修复 bug、写测试前都必须遵守本文档。
>
> 处理任一服务前，必须先读：
>
> 1. [`AGENTS.md`](./AGENTS.md)
> 2. [`ENGINEERING_GUIDE.md`](./ENGINEERING_GUIDE.md)
> 3. [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md)
> 4. [`SECURITY_HARDENING.md`](./SECURITY_HARDENING.md)
> 5. 本文档

---

## 1. 核心原则

### 1.1 业务代码禁止内置 mock / stub / fake 数据

生产路径代码中禁止出现以下行为：

- 写死行情价格；
- 写死技术指标；
- 写死基本面数据；
- 写死新闻 / 舆情内容；
- Redis / 数据源不可用时自动返回看似真实的 stub 数据；
- LLM / trading / orchestrator 基于 mock 数据继续写 live 状态；
- 为了让页面或接口“看起来有数据”而在业务代码中构造假数据。

禁止示例：

```python
_STUB_INDICATORS = {
    "price": 2350.0,
    "ma_fast": 2345.0,
}

if redis_unavailable:
    return dict(_STUB_INDICATORS)
```

正确行为：

```text
数据源不可用
  → 返回 data_unavailable / degraded
  → 不写 live score / 不触发交易决策
  → healthz 反映异常
  → watchdog / alert 告警
```

### 1.2 测试数据只能从正式上游服务或测试夹具注入

测试时可以使用 mock 数据，但只能通过正式数据流入口注入：

```text
ctp_tick / stock_tick / intel_collector
```

或这些服务的测试 adapter / fixture。

禁止在下游业务服务中写测试数据兜底：

```text
llm_scorer 内置假新闻 / 假指标        ❌
orchestrator 内置假持仓 / 假订单       ❌
trading_agent 内置假行情 / 假 AI 分数  ❌
dashboard 内置假接口结果且不登记 REQ  ❌
```

允许的测试方式：

```text
测试用例 → 向 ctp_tick 测试输入投递 tick
测试用例 → 向 stock_tick 测试输入投递股票行情
测试用例 → 向 intel_collector 测试输入投递新闻
这些服务 → 按正式 Redis Stream / KV / contracts 写出
下游服务 → 只按正式路径消费
```

### 1.3 关注合约 / 股票不能写死在代码里

禁止在业务代码中写死：

```text
MA2509
MA2601
某个固定股票代码
固定 watchlist
```

允许出现在：

- 文档示例；
- 测试 fixture；
- 本地开发 `.env.example` 示例值；
- 数据库初始化示例，但不能作为生产默认关注列表。

生产关注标的必须来自用户配置：

```text
用户在 dashboard 指定
  ↓
orchestrator 校验并写入持久化配置
  ↓
watchlist/config store 成为唯一事实来源
  ↓
ctp_tick / stock_tick / intel_collector / trading_agent / llm_scorer 订阅或读取该配置
```

---

## 2. 正式数据流

### 2.1 行情数据流

期货行情：

```text
CTP / 行情源
  ↓
ctp_tick
  ↓
Redis Streams / KV（正式 key schema）
  ↓
trading_agent / llm_scorer / orchestrator / db_writer
```

股票行情：

```text
股票行情源
  ↓
stock_tick
  ↓
Redis Streams / KV（正式 key schema）
  ↓
trading_agent / llm_scorer / orchestrator / db_writer
```

下游服务不得绕过 `ctp_tick` / `stock_tick` 自己构造行情。

### 2.2 舆情 / 新闻数据流

```text
外部新闻 / RSS / 资讯源
  ↓
intel_collector
  ↓
Redis Streams / cleaned intel / news cache（正式 key schema）
  ↓
llm_scorer
  ↓
write:llm_analysis / sentiment index / ai_score
  ↓
db_writer / trading_agent / orchestrator
```

`llm_scorer` 不负责抓取原始新闻，也不内置假新闻。

### 2.3 AI 评分数据流

```text
正式行情 + 正式指标 + 正式舆情
  ↓
llm_scorer
  ↓
ai_score:{symbol}:live / shadow / emergency
write:score
  ↓
trading_agent / db_writer / orchestrator
```

如果输入数据不可用：

```text
不写 live score
写 degraded / data_unavailable 状态
保留错误原因用于 healthz / metrics / audit
```

---

## 3. Watchlist / 关注标的设计原则

### 3.1 Watchlist 是配置，不是代码常量

关注标的必须是用户配置，而不是代码常量。

目标流程：

```text
dashboard 管理页面
  ↓ REST
orchestrator
  ↓ 校验合约 / 股票合法性
persistent config store（数据库 / Redis durable config / 文件配置，择一明确）
  ↓ 发布 watchlist_changed 事件
各服务刷新订阅
```

### 3.2 Watchlist 唯一事实来源

必须明确一个唯一事实来源，例如：

```text
instrument_watchlist 表 / config:watchlist key / orchestrator-managed config
```

禁止多个服务各自维护自己的默认关注列表。

### 3.3 服务职责

| 服务 | Watchlist 职责 |
|---|---|
| dashboard_web | 用户查看 / 修改关注标的 |
| orchestrator | 校验、持久化、发布变更事件 |
| ctp_tick | 根据期货 watchlist 订阅行情 |
| stock_tick | 根据股票 watchlist 订阅行情 |
| intel_collector | 根据 watchlist 过滤 / 关联新闻 |
| llm_scorer | 根据 watchlist 对有正式数据的标的评分 |
| trading_agent | 只对允许交易 / 关注标的执行策略 |
| watchdog | 检查各服务 watchlist 版本是否一致 |

---

## 4. 失败时的正确行为

### 4.1 数据源不可用

当正式数据源不可用时：

| 场景 | 正确行为 |
|---|---|
| Redis 不可用 | 服务 degraded/unhealthy，不写 live |
| tick 缺失 | 对应 symbol 不评分 / 不交易 |
| indicator 缺失 | 不写 live，或降级为 shadow 记录 |
| news 缺失 | 舆情 Agent skipped，不构造假新闻 |
| watchlist 缺失 | 服务启动失败或 degraded，不使用代码默认合约 |
| LLM 输出不可解析 | 重试 / fallback provider，仍失败则不写 live |

### 4.2 禁止 fail-open

交易相关链路必须 fail-closed：

```text
不确定 → 不写 live / 不交易 / 降级 / 告警
```

禁止：

```text
不确定 → 用默认值继续
不确定 → 用 stub 继续
不确定 → 当成 0 分但仍写 live
```

---

## 5. 测试策略

### 5.1 单元测试

单元测试可以 mock 外部依赖，但 mock 必须留在：

```text
tests/
测试 fixture
测试 adapter
```

不得进入生产模块。

### 5.2 集成测试

集成测试应按正式数据流投递数据：

```text
测试 → ctp_tick 测试入口 → Redis 正式 key/stream → 下游服务
测试 → stock_tick 测试入口 → Redis 正式 key/stream → 下游服务
测试 → intel_collector 测试入口 → Redis 正式 key/stream → llm_scorer
```

### 5.3 Dashboard 测试

Dashboard 如果接口暂未实现：

- 必须在 `services/orchestrator/API_REQUESTS.md` 登记 REQ；
- mock 只能放在前端测试 / Storybook / demo fixture；
- 生产页面不能静默使用假数据冒充真实接口。

---

## 6. 需要立即清理的已知风险

### 6.1 llm_scorer 内置 stub 数据

当前 `llm_scorer` 已发现内置 stub 数据风险，已记录在：

```text
services/llm_scorer/REVIEW_ISSUES.md
LLM-REVIEW-005 Redis 不可用 / 数据为空时使用 stub 假数据写 live score
```

该问题必须作为 P0 修复。

### 6.2 固定 symbol 默认值

当前部分代码 / 配置中存在 `MA2509` 示例或默认值。后续修复时应区分：

| 场景 | 是否允许 |
|---|---|
| README 示例 | ✅ 可以，但必须标明示例 |
| `.env.example` 示例 | ✅ 可以，但生产需覆盖 |
| 单元测试 fixture | ✅ 可以 |
| 生产业务代码默认关注合约 | ❌ 禁止 |
| 数据源缺失时 fallback 到固定合约 | ❌ 禁止 |

---

## 7. Agent 实现要求

后续 agent 修改代码时必须遵守：

1. 不新增生产路径 mock/stub/fake 数据；
2. 不新增硬编码关注合约；
3. 不让下游服务绕过正式上游数据源；
4. 数据不可用时 fail-closed；
5. 测试数据从测试入口或正式上游服务注入；
6. 新增配置必须写入 `.env.example`，但不得把示例值作为生产默认事实；
7. 新增接口必须同步 README / API_REQUESTS / contracts；
8. 所有 live 写入必须能追溯数据来源。

---

## 8. 变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-19 | 初版：明确禁止业务代码内置 mock/stub 数据、禁止写死关注标的、要求测试数据通过 ctp_tick/stock_tick/intel_collector 正式数据流注入 |
