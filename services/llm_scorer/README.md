# services/llm_scorer — LLM 评分服务

> 本服务属于 arbitrage-system 微服务集合的一员。
> 在动手前必读：[`/ENGINEERING_GUIDE.md`](../../ENGINEERING_GUIDE.md) + [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)
> 重点章节：架构机制 4（事件驱动 AI 紧急触发）+ 机制 6 Stage 4（语义理解）

---

## 服务定位（一句话）

LLM 智能层。负责**两件事**：
1. **定时综合评分**：每 5 分钟跑多 Agent 协作，产出 AI 评分写入 Redis
2. **逐条情感分析**：订阅 `news_stream`，对每条新闻打 LLM 情感分（机制 6 Stage 4）

---

## 职责

### 做什么 ✅
- 每 5 分钟触发一轮综合评分（market + technical + fundamentals + fuser 四个 Agent）
- 调用 DeepSeek-V3 → 智谱 GLM-4 二级熔断
- 写 `ai_score:MA2509:live` 和 `ai_score:MA2509:shadow` 双轨
- 订阅 `news_stream`，对每条新闻调 LLM 打情感分（输出存 InfluxDB `llm_analysis`）
- 提供 `/emergency_analysis` 端点：trading_agent 检测剧烈波动时调用，生成紧急评分
- 聚合 Stage 5：合成时间窗口的舆情指数 `sentiment:MA2509:current`

### 不做什么 ❌
- 不做交易决策
- 不抓爬虫数据（intel_collector 的事）
- 不直接写 InfluxDB（走 db_writer）
- 不缓存 LLM 结果给业务决策用（每次调 LLM 是真实调用，结果存 InfluxDB 后回测用）

---

## 内部结构

```
llm_scorer/
├── Dockerfile                  ← python:3.12-slim
├── pyproject.toml              ← uv 管理依赖
├── README.md                   ← 本文件
├── .env.example
├── main.py                     ← 入口：启动定时调度 + Stream 订阅 + FastAPI
├── domain/                     ← 业务逻辑（零外部依赖）
│   ├── agents/                 多 Agent 协作
│   │   ├── base.py               Agent 基类
│   │   ├── market_context.py     市场环境 Agent
│   │   ├── technical.py          技术分析 Agent
│   │   ├── fundamentals.py       基本面 Agent（甲醇专项）
│   │   └── fuser.py              融合多 Agent 输出
│   ├── sentiment/              新闻情感打分
│   │   └── single_news_scorer.py
│   ├── prompt_templates.py     Prompt 模板（含版本号）
│   └── scoring.py              评分聚合纯逻辑（机制 6 Stage 5）
├── adapters/                   ← 外部 IO
│   ├── llm_gateway.py          ⭐ 二级熔断 (DeepSeek → GLM → 停机)
│   ├── data_fetcher.py         甲醇库存/基差/原油联动数据
│   ├── redis_writer.py         写 ai_score:MA2509:* 系列
│   └── config.py
├── api/                        ← FastAPI
│   ├── healthz.py                /healthz
│   ├── metrics.py                /metrics
│   └── emergency_analysis.py     ⭐ POST /emergency_analysis
└── tests/{unit, integration}/
```

---

## 关键机制：LLM 二级熔断

```
DeepSeek-V3 (主)
    │ HTTP 500 或 timeout 30s
    ▼
智谱 GLM-4 (备)
    │ 又挂了
    ▼
停机告警（推钉钉）
    │
    ▼
trading_agent 检测到 ai_score 超过 30 分钟未更新
    │
    ▼
切「AI=0 模式」（只平仓不开新仓）
```

**实现位置**：`adapters/llm_gateway.py`

```python
class LLMGateway:
    async def chat(self, prompt: str, model_chain: list = None):
        chain = model_chain or ["deepseek-v3", "glm-4"]
        for model in chain:
            try:
                return await self._call_with_timeout(model, prompt, timeout=60)
            except (Timeout, HTTPError) as e:
                logger.warn(f"{model} failed: {e}, falling back...")
                continue
        # 全部失败
        await alert_dingtalk("🚨 所有 LLM 都挂了，停止评分")
        raise LLMAllDownError()
```

---

## 关键机制：紧急分析端点

trading_agent 检测到剧烈波动（1 分钟价格涨跌幅 > 1%）时调用：

```
POST /emergency_analysis
Content-Type: application/json

{
  "symbol": "MA2509",
  "trigger_reason": "price_spike",
  "volatility": {
    "price_change_pct_1m": 1.2,
    "volume_multiplier": 3.5,
    "one_sided_ratio": 0.92
  },
  "recent_ticks": [...]   // 最近 60 秒 Tick
}
```

llm_scorer 处理流程：
1. 立即抓最近 30 分钟相关新闻（含 sentiment_monitor 数据）
2. 用 `EMERGENCY_PROMPT` 调 LLM（timeout 30s）
3. 写 `ai_score:MA2509:emergency`（TTL 10 分钟）
4. 推钉钉告警

详见 ARCHITECTURE.md 机制 4。

---

## 数据流（机制 6 Stage 4-5）

### 新闻情感分析（订阅 news_stream）

```
intel_collector → XADD news_stream (清洗后的新闻)
                          ↓
               llm_scorer 订阅消费
                          ↓
               LLM 情感打分（DeepSeek/GLM）
                          ↓
               写 InfluxDB llm_analysis bucket（永久）
                          ↓
               异步聚合 sentiment_index（每分钟一个快照）
                          ↓
               写 InfluxDB sentiment_index bucket
                          ↓
               写 Redis sentiment:MA2509:current（TTL 1h）
```

### 综合评分（每 5 分钟定时）

```
定时器触发
    ↓
data_fetcher 抓取最新数据（库存 / 基差 / 原油 / 技术指标）
    ↓
4 个 Agent 并行分析（market / technical / fundamentals / fuser）
    ↓
fuser 综合输出 AIScore 消息
    ↓
写 Redis: ai_score:MA2509:live (alpha=0.2 影响下单)
        + ai_score:MA2509:shadow (仅记录用于回测对比)
    ↓
写 InfluxDB ai_score_history bucket（永久）
```

---

## 对外接口

### 暴露的端点

| 类型 | 端口 | 路径 | 用途 |
|---|---|---|---|
| HTTP | 5000 | `/healthz` | 健康检查（含 LLM 可用性） |
| HTTP | 5000 | `/metrics` | Prometheus 指标 |
| HTTP | 5000 | `/emergency_analysis` | ⭐ trading_agent 触发紧急分析 |

### 写入的 Redis

| 目的 | Key/Stream | 频率 | 内容 |
|---|---|---|---|
| 实际生效评分 | `ai_score:MA2509:live` (KV) | 5min | AIScore proto，alpha=0.2 |
| 影子评分 | `ai_score:MA2509:shadow` (KV) | 5min | 同上，仅记录用 |
| 紧急评分 | `ai_score:MA2509:emergency` (KV) | 触发时 | TTL 10min |
| 舆情指数 | `sentiment:MA2509:current` (KV) | 每分钟 | TTL 1h |
| 持久化流 | `write:score` / `write:llm_analysis` (Stream) | 实时 | db_writer 写 InfluxDB |
| 服务注册 | `services:llm_scorer` (KV) | 10s 续约 | TTL 30s |

### 消费的 Redis

| 来源 | Stream | 行为 |
|---|---|---|
| `news_stream` | XREADGROUP | 每条新闻调 LLM 情感分 |

---

## 依赖

| 依赖 | 类型 | 必需 |
|---|---|---|
| Redis 7.2 | 内部服务 | ✅ |
| DeepSeek API | 外部 | ✅ |
| 智谱 GLM-4 API | 外部 | 🟡 备用 |
| `contracts/ai_score.proto` | 内部契约 | ✅ |
| `contracts/intel_event.proto` | 内部契约 | ✅ |

---

## 配置（关键环境变量）

```bash
SERVICE_NAME=llm_scorer
HEALTH_PORT=5000

REDIS_ADDR=redis:6379

# LLM API
DEEPSEEK_API_KEY=sk-xxx
DEEPSEEK_BASE_URL=https://api.deepseek.com/v1
DEEPSEEK_MODEL=deepseek-chat

ZHIPU_API_KEY=xxx
ZHIPU_MODEL=glm-4

# 评分调度
SCORE_INTERVAL=300         # 5 分钟
SHADOW_ENABLED=true
SYMBOL=MA2509
ALPHA=0.2                  # AI 评分权重（最大 ±20% 仓位影响）

# Prompt 版本号（用于审计追溯）
PROMPT_VERSION=v1.0

LOG_LEVEL=info
```

---

## 启动方式

```bash
# 本地开发
cd services/llm_scorer/
uv sync
python main.py

# Docker
docker compose --profile ai-only up -d llm_scorer
```

---

## 测试

```bash
# Unit 测试（用 mock LLM）
pytest tests/unit/

# Integration（真 DeepSeek，会花钱）
pytest tests/integration/ -m needs_llm
```

⚠️ 测试时不要把 PROMPT_VERSION 改了——这会影响 InfluxDB 历史评分的可追溯性。

---

## 已知限制 / 风险点

### 1. ⚠️ LLM 输出不稳定

DeepSeek 偶尔返回非 JSON 格式 / 字段缺失。
解决：
- 严格 JSON Schema 校验，失败则重试
- 重试 3 次仍失败 → 走 GLM-4 兜底

### 2. ⚠️ Prompt 版本必须固化

回测时必须用**当时记录的 LLM 输出**（机制 6 时间倒带原则），不能重新调 LLM。
所以 `PROMPT_VERSION` 改动必须**升版本号**（不可覆盖旧版本）。

### 3. ⚠️ 评分老化是 trading_agent 的责任

llm_scorer 不管下游怎么用评分。
trading_agent 自己根据 `timestamp` 做老化（< 15min/15-30min/>30min decay 1.0/0.5/0.0）。

### 4. ⚠️ 紧急分析有 5 分钟防抖

trading_agent 在客户端做防抖：5 分钟内最多触发一次紧急分析。
否则 LLM 调用频率失控。

### 5. ⚠️ Python 服务必须 mem_limit

docker-compose 中强制 `mem_limit: 1g`，防 Python 内存泄漏蔓延。

---

## 与 ARCHITECTURE.md 的关联

- 机制 4：事件驱动 AI 介入（紧急分析）
- 机制 6 Stage 4：语义理解（核心职责）
- 机制 6 Stage 5：聚合（舆情指数）
- 关键技术选型：DeepSeek OpenAI 兼容 SDK（不用 litellm）

---

## AI Agent 准则

- ✅ 改 prompt → 必须升 `PROMPT_VERSION`（影响审计可追溯性）
- ✅ 改 Agent 逻辑 → unit 测试用 mock LLM 覆盖
- ❌ 不要在 `domain/` 里 import openai / httpx / redis
- ❌ 不要在回测代码里重新调 LLM 分析旧新闻（**未来函数泄露**！）
- ❌ 不要写 InfluxDB（走 db_writer）

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版 |
