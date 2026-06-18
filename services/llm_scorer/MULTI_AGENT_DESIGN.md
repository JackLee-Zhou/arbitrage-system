# llm_scorer 多 Agent 架构设计

> **状态**：设计阶段
> **决策日期**：2026-06-17
> **技术选型**：OpenAI Agents SDK + DeepSeek API

---

## 1. 背景与动机

### 当前问题

llm_scorer 当前使用**单次 LLM 调用**完成综合评分：

```
一个 prompt 塞入所有分析维度（市场环境 + 技术面 + 基本面）→ 一次调用 → 输出评分
```

问题：
- 单个 prompt 承载过多分析任务，LLM 输出质量下降
- 各维度分析互相干扰（技术面信号可能影响基本面判断）
- 无法对单个维度做独立优化（如舆情 prompt 改了可能影响技术面输出）
- 缺少交叉验证，单次调用的盲区无法被发现

### 目标

改为**多 Agent 并行分析 + 汇总**模式，提升分析准确性：

```
3 个专精 Agent 并行分析 → 各自独立输出 → 1 个汇总 Agent 交叉验证 → 最终评分
```

---

## 2. 技术选型

### 选择：OpenAI Agents SDK

| 方案 | 选择理由 / 排除理由 |
|---|---|
| **OpenAI Agents SDK** ✅ | 轻量 + Handoffs 原生支持 + DeepSeek 兼容 API + Guardrails 内置 |
| PydanticAI + 自研 | 备选方案，如果 OpenAI Agents SDK 对 DeepSeek 兼容不好则退回 |
| LangGraph | 过重，引入 LangChain 生态（~50MB），当前流程用不上状态机/断点恢复 |
| CrewAI | 角色扮演浪费 token，输出结构化弱 |
| Claude Agent SDK | 绑死 Claude 模型，与 DeepSeek/GLM 技术栈不兼容 |
| AutoGen | 对话式模型 token 消耗不可控 |

### DeepSeek 兼容性验证

DeepSeek 提供 OpenAI 兼容 API，OpenAI Agents SDK 理论上可直接使用：

```python
from openai import AsyncOpenAI

client = AsyncOpenAI(
    base_url="https://api.deepseek.com/v1",
    api_key="sk-xxx",
)
```

**需验证的特性**：
- [ ] 基础对话调用
- [ ] Function calling / tool use
- [ ] JSON mode（结构化输出）
- [ ] 并发调用稳定性

**验证方法**：先写最小 demo 跑通，确认无兼容性问题再集成。

---

## 3. 架构设计

### 3.1 Agent 分工

| Agent | 职责 | 输入 | 输出 |
|---|---|---|---|
| **sentiment_agent** | 舆情分析 | 最近 30 分钟新闻列表 | score [-1,+1] + confidence [0,1] + factors |
| **fundamental_agent** | 基本面分析 | 甲醇库存/开工率/下游需求/港口到货/基差 | score + confidence + factors |
| **technical_agent** | 技术面分析 | K 线/均线/布林带/ATR/量比/支撑阻力 | score + confidence + factors |
| **aggregator** | 汇总决策 | 三个维度的分析结果 | 最终 score + confidence + breakdown |

### 3.2 数据流

```
定时器触发（每 5 分钟）
    │
    ├── data_fetcher 抓取最新数据
    │     ├→ 最近新闻（Redis / intel_collector）
    │     ├→ 基本面数据（外部 API / InfluxDB）
    │     └→ 技术指标（Redis indicator:* / trading_agent）
    │
    ├── 3 个 Agent 并行执行（asyncio.gather）
    │     ├→ sentiment_agent(news)
    │     ├→ fundamental_agent(fundamentals)
    │     └→ technical_agent(indicators)
    │
    ├── 汇总 Agent（串行，依赖上面三个结果）
    │     └→ aggregator(sentiment, fundamental, technical)
    │
    └→ 输出
          ├→ 写 Redis: ai_score:MA2509:live
          ├→ 写 Redis: ai_score:MA2509:shadow
          └→ 写 InfluxDB: ai_score_history
```

### 3.3 目录结构变更

```
llm_scorer/
├── domain/
│   ├── agents/                        ← 改造：引入 OpenAI Agents SDK
│   │   ├── sentiment_agent.py           舆情 Agent 定义 + prompt
│   │   ├── fundamental_agent.py         基本面 Agent 定义 + prompt
│   │   ├── technical_agent.py           技术面 Agent 定义 + prompt
│   │   ├── aggregator.py                汇总 Agent 定义 + prompt
│   │   ├── tools.py                     Agent 可调用的工具函数
│   │   ├── guardrails.py               输入/输出校验
│   │   └── runner.py                   ⭐ 并行编排 + 汇总逻辑
│   ├── prompts/                       ← 新增：prompt 模板文件
│   │   ├── sentiment.txt
│   │   ├── fundamental.txt
│   │   ├── technical.txt
│   │   └── aggregator.txt
│   ├── sentiment/                     ← 不变
│   └── scoring.py                     ← 不变
├── adapters/
│   ├── llm_gateway.py                 ← 改造：适配 OpenAI Agents SDK
│   └── ...
└── tests/
    └── unit/
        └── agents/                    ← 新增：Agent 单元测试
            ├── test_sentiment_agent.py
            ├── test_fundamental_agent.py
            ├── test_technical_agent.py
            ├── test_aggregator.py
            └── test_runner.py
```

---

## 4. 核心实现

### 4.1 Agent 定义（以 sentiment_agent 为例）

```python
# domain/agents/sentiment_agent.py
from openai.agents import Agent, function_tool
from .tools import get_latest_news

SENTIMENT_PROMPT = """你是甲醇期货舆情分析师。只分析新闻情绪，不管技术面和基本面。

分析维度：
1. 宏观政策（国常会/发改委/环保限产）
2. 行业新闻（甲醇开工率/港口到货/下游 MTO 装置）
3. 突发事件（地缘政治/自然灾害/安全事故）

输出 JSON：
{
  "score": float,        // [-1, +1]，正=看多，负=看空
  "confidence": float,   // [0, 1]，信息充分度
  "factors": [str],      // 关键因素列表（3-5 条）
  "reasoning": str       // 推理过程（用于审计追溯）
}"""

sentiment_agent = Agent(
    name="sentiment_analyst",
    instructions=SENTIMENT_PROMPT,
    tools=[get_latest_news],
    model="deepseek-chat",
)
```

### 4.2 工具函数

```python
# domain/agents/tools.py
from openai.agents import function_tool

@function_tool
def get_latest_news(symbol: str, minutes: int = 30) -> list[dict]:
    """获取最近 N 分钟的新闻列表"""
    # 从 Redis 读取
    return news_list

@function_tool
def get_fundamentals(symbol: str) -> dict:
    """获取基本面数据（库存/开工率/基差）"""
    return fundamentals

@function_tool
def get_technical_indicators(symbol: str) -> dict:
    """获取技术指标（MA/布林带/ATR/量比）"""
    return indicators

@function_tool
def get_recent_ticks(symbol: str, count: int = 60) -> list[dict]:
    """获取最近 N 个 Tick"""
    return ticks
```

### 4.3 并行编排（核心）

```python
# domain/agents/runner.py
import asyncio
import logging
from openai.agents import Runner
from .sentiment_agent import sentiment_agent
from .fundamental_agent import fundamental_agent
from .technical_agent import technical_agent
from .aggregator import aggregator

logger = logging.getLogger(__name__)

async def run_multi_agent_analysis(symbol: str) -> dict:
    """
    多 Agent 并行分析 + 汇总。
    
    流程：
    1. 3 个专精 Agent 并行执行（asyncio.gather）
    2. 汇总 Agent 综合三方结果
    3. 返回结构化评分
    """
    prompt = f"分析 {symbol} 当前市场状况"

    # Step 1: 并行分析
    logger.info(f"开始多 Agent 并行分析: {symbol}")
    sentiment_result, fundamental_result, technical_result = await asyncio.gather(
        Runner.run(sentiment_agent, prompt),
        Runner.run(fundamental_agent, prompt),
        Runner.run(technical_agent, prompt),
    )

    # Step 2: 汇总
    summary_prompt = f"""汇总以下三个维度的分析结果：

【舆情分析】
{sentiment_result.final_output}

【基本面分析】
{fundamental_result.final_output}

【技术面分析】
{technical_result.final_output}

请给出最终评分。"""

    final = await Runner.run(aggregator, summary_prompt)

    logger.info(f"多 Agent 分析完成: {symbol}")
    return {
        "sentiment": sentiment_result.final_output,
        "fundamental": fundamental_result.final_output,
        "technical": technical_result.final_output,
        "final": final.final_output,
    }
```

### 4.4 Guardrails（输入校验）

```python
# domain/agents/guardrails.py
from openai.agents import InputGuardrail, GuardrailFunctionOutput

@InputGuardrail
async def validate_trading_hours(ctx, agent, input):
    """检查是否在交易时段（防止非交易时段浪费 token）"""
    from datetime import datetime, time as dt_time
    now = datetime.now().time()
    
    # 甲醇交易时段：09:00-11:30, 13:30-15:00, 21:00-23:00
    trading_hours = [
        (dt_time(9, 0), dt_time(11, 30)),
        (dt_time(13, 30), dt_time(15, 0)),
        (dt_time(21, 0), dt_time(23, 0)),
    ]
    
    in_hours = any(start <= now <= end for start, end in trading_hours)
    
    return GuardrailFunctionOutput(
        tripwire_triggered=not in_hours,
        message="非交易时段，跳过分析" if not in_hours else "OK",
    )

@InputGuardrail
async def validate_symbol(ctx, agent, input):
    """检查 symbol 是否合法"""
    valid_symbols = ["MA2509", "MA2601"]  # 从配置读取
    # ... 校验逻辑
    return GuardrailFunctionOutput(tripwire_triggered=False, message="OK")
```

---

## 5. Token 优化策略

### 5.1 分层调用（省 token 的关键）

```
第一层：快速筛选（短 prompt / 便宜模型）
  └→ 判断"有没有分析价值"
  └→ 无信号 → 跳过对应 Agent

第二层：深度分析（专精 prompt / deepseek-chat）
  └→ 只分析有价值的维度
```

```python
async def run_smart_analysis(symbol: str) -> dict:
    # 快速预筛
    news = get_latest_news_sync(symbol, minutes=30)
    has_news = len(news) > 0 and is_recent(news[0])
    
    tasks = []
    
    # 舆情：有新新闻才分析
    if has_news:
        tasks.append(Runner.run(sentiment_agent, f"分析舆情：{news}"))
    else:
        tasks.append(_noop_result())  # 跳过，不消耗 token
    
    # 技术面：总是分析（变化频繁）
    tasks.append(Runner.run(technical_agent, f"分析技术面：{symbol}"))
    
    # 基本面：每天只分析几次（变化慢）
    if should_analyze_fundamental(symbol):  # 每天 8:30, 12:00, 20:30
        tasks.append(Runner.run(fundamental_agent, f"分析基本面：{symbol}"))
    else:
        tasks.append(_noop_result())
    
    results = await asyncio.gather(*tasks)
    # ... 汇总
```

### 5.2 缓存（相同输入不重复调用）

```python
from functools import lru_cache
import hashlib
import json

# 5 分钟内相同新闻不重复分析
_analysis_cache = {}

async def cached_sentiment(symbol: str, news_hash: str) -> dict:
    cache_key = f"{symbol}:{news_hash}"
    if cache_key in _analysis_cache:
        cached_time, cached_result = _analysis_cache[cache_key]
        if time.time() - cached_time < 300:  # 5 分钟 TTL
            return cached_result
    
    result = await Runner.run(sentiment_agent, ...)
    _analysis_cache[cache_key] = (time.time(), result)
    return result
```

### 5.3 降级策略

```
正常模式：3 Agent 并行 + 汇总（4 次 LLM 调用）
  ↓ 触发降级条件
降级模式：单 Agent 一次调用（1 次 LLM 调用，省 75% token）
```

降级触发条件：
- DeepSeek API 超时 / 限流（HTTP 429）
- 当前是非活跃时段（23:00-02:00，夜盘收盘后）
- 连续 3 次评分结果变化 < 0.05（市场平稳，无需多维度）

```python
async def run_with_fallback(symbol: str) -> dict:
    try:
        # 正常模式：多 Agent
        return await asyncio.wait_for(
            run_multi_agent_analysis(symbol),
            timeout=45.0,  # 45s 超时
        )
    except asyncio.TimeoutError:
        logger.warning(f"多 Agent 分析超时，降级为单次调用")
        return await run_single_agent_fallback(symbol)

async def run_single_agent_fallback(symbol: str) -> dict:
    """降级：单 Agent 一次调用"""
    result = await Runner.run(fallback_agent, f"综合分析 {symbol}")
    return {"final": result.final_output, "mode": "fallback"}
```

---

## 6. Token 消耗估算

### 单次评分对比

| 方案 | LLM 调用次数 | 估算 Token | 耗时 |
|---|---|---|---|
| 当前：单次调用 | 1 | ~2,000 | ~15s |
| 多 Agent（3+1） | 4 | ~8,000 | ~20-25s |
| 多 Agent + 优化（跳过无信号维度） | 2-3 | ~5,000 | ~18-22s |
| 降级模式 | 1 | ~2,500 | ~15s |

### 每日消耗估算

| 场景 | 评分次数/天 | Token/次 | 日消耗 |
|---|---|---|---|
| 交易时段（9:00-15:00 + 21:00-23:00） | ~100 次 | ~8,000 | ~800,000 |
| 优化后（跳过无信号维度） | ~100 次 | ~5,000 | ~500,000 |
| 降级模式占比 20% | ~100 次 | ~4,400 | ~440,000 |

DeepSeek 定价参考（2026 年）：
- deepseek-chat: ~¥1/百万 token（输入）+ ¥2/百万 token（输出）
- 日消耗 ~500,000 token ≈ ¥1-2/天

---

## 7. 与现有机制的兼容

### 不变的部分

| 组件 | 是否变更 | 说明 |
|---|---|---|
| `ai_score` Redis Key | ❌ 不变 | 对外契约不变 |
| `AIScore` protobuf | ❌ 不变 | trading_agent 不感知内部改动 |
| LLM 二级熔断 | ❌ 不变 | llm_gateway.py 逻辑保留 |
| 紧急分析端点 | ❌ 不变 | `/emergency_analysis` 接口不变 |
| 影子评分 | ❌ 不变 | shadow 轨继续写 |
| InfluxDB 写入 | ❌ 不变 | 走 db_writer |

### 变更的部分

| 组件 | 变更内容 |
|---|---|
| `domain/agents/` | 从自研基类 → OpenAI Agents SDK 定义 |
| `domain/agents/runner.py` | 新增：并行编排 + 汇总逻辑 |
| `domain/agents/tools.py` | 新增：Agent 可调用的工具函数 |
| `domain/agents/guardrails.py` | 新增：输入校验 |
| `pyproject.toml` | 新增依赖：`openai-agents` |
| `adapters/llm_gateway.py` | 适配 OpenAI Agents SDK 的 client 初始化 |

---

## 8. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|---|---|---|
| DeepSeek 对 OpenAI Agents SDK 兼容不完整 | Handoffs/Tools 不能用 | 退回 PydanticAI + asyncio.gather 方案 |
| Token 消耗超预期 | 成本增加 | 分层调用 + 缓存 + 降级策略 |
| 并行调用被 DeepSeek 限流 | 评分延迟 | 加重试 + 降级为单次调用 |
| 多 Agent 结果不一致 | 汇总 Agent 纠结 | 设置 confidence 阈值，低于阈值降权 |
| Agent 输出非 JSON | 解析失败 | Pydantic 校验 + 重试 3 次 |

---

## 9. 实施计划

### Phase 1：验证兼容性（1 天）

- [ ] `pip install openai-agents`
- [ ] 写最小 demo：DeepSeek + OpenAI Agents SDK 单 Agent 调用
- [ ] 验证 function calling 是否正常
- [ ] 验证并发调用（3 个并行）是否稳定
- [ ] 记录兼容性问题，决定是否继续用此方案

### Phase 2：实现核心（3-5 天）

- [ ] 定义 3 个专精 Agent（sentiment / fundamental / technical）
- [ ] 实现工具函数（tools.py）
- [ ] 实现并行编排（runner.py）
- [ ] 实现汇总 Agent（aggregator）
- [ ] 单元测试（用 mock LLM）

### Phase 3：集成优化（2-3 天）

- [ ] 接入 llm_gateway（二级熔断）
- [ ] 实现 Guardrails（交易时段校验）
- [ ] 实现分层调用（快速筛选 + 深度分析）
- [ ] 实现缓存（相同输入不重复调用）
- [ ] 实现降级策略

### Phase 4：测试上线（1-2 天）

- [ ] Integration 测试（真 DeepSeek API）
- [ ] Token 消耗实测
- [ ] 对比新旧方案的评分质量
- [ ] 更新 README

---

## 10. 决策记录

| 日期 | 决策 | 理由 |
|---|---|---|
| 2026-06-17 | 选择 OpenAI Agents SDK | 轻量 + DeepSeek 兼容 + Handoffs 原生支持 |
| 2026-06-17 | 不选 LangGraph | 过重，引入 LangChain 生态不值得 |
| 2026-06-17 | 不选 Claude Agent SDK | 绑死 Claude 模型，与 DeepSeek/GLM 技术栈冲突 |
| 2026-06-17 | Agent 放在 llm_scorer 内部 | 对外契约不变，trading_agent 不感知 |
| 2026-06-17 | intel_collector 保持独立 | 可能部署到香港 |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-17 | 初版：多 Agent 架构设计 + OpenAI Agents SDK 选型 |
