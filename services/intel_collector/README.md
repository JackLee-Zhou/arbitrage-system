# services/intel_collector — 舆情统一采集服务

> 本服务属于 arbitrage-system 微服务集合的一员。
> 在动手前必读：[`/ENGINEERING_GUIDE.md`](../../ENGINEERING_GUIDE.md) + [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)
> 重点章节：架构机制 5（舆情监控三层数据源）+ 机制 6（5 阶段清洗管道）

---

## 服务定位（一句话）

抓取多渠道舆情原始数据（RSS / HTML / Telegram），完成 **Stage 1-3 清洗**（抓取 + 解析 + 清洗），写入 Redis Stream 供 llm_scorer 做语义分析。

**不做语义分析**（那是 llm_scorer 的事，对应 ARCHITECTURE.md 机制 6 的 Stage 4）。

---

## 服务来源

v2.1 架构合并：原 `domestic_intel` / `overseas_intel` / `social_intel` 三个 Python 服务合并为一个，通过 YAML 配置驱动数据源差异。

理由：三个服务都是 Python + 都做爬虫 + 共享清洗代码，**配置驱动比拆分更优**。

---

## 职责

### 做什么 ✅
- **Stage 1 抓取**：根据配置抓取 RSS / HTML / Telegram 群组消息
- **Stage 2 解析**：feedparser / BeautifulSoup / telethon 内置解析
- **Stage 3 清洗**：归一化 + 关键词过滤 + SimHash 跨源去重
- 写入 Redis Stream `news_stream` 供 llm_scorer 消费
- 写入 Redis Stream `write:sentiment` 供 db_writer 持久化（含 raw_intel + parsed_intel + cleaned_intel 三层）
- 服务自报家门到 Redis（`services:intel_collector` + 30s TTL）

### 不做什么 ❌
- **不调用 LLM**（那是 llm_scorer 的事，Stage 4）
- 不做情感分析、实体识别等语义工作
- 不直接写 InfluxDB（统一走 db_writer）
- 不做舆情指数聚合（Stage 5 也是 llm_scorer 做）

---

## 三层数据源体系

### Layer 1: 国内源（P1 启用，不走代理）

V1（M4）：仅财联社化工 RSS（5 分钟一次）
V2（M8）：+ 东方财富股吧（甲醇）+ 雪球甲醇话题 + 卓创资讯（如订阅）

### Layer 2: 海外源（P3 启用，走香港代理）

- OilPrice.com（原油新闻 RSS）
- Reuters Energy RSS
- Methanex（甲醇龙头公司公告）
- IEA（国际能源署月报）

### Layer 3: 社交源（V4 启用，走香港代理）

- Telegram 公开群（化工 / 能源）
- ❌ 明确不做：Discord、X (Twitter)

---

## 内部结构

```
intel_collector/
├── Dockerfile                  ← python:3.12-slim
├── pyproject.toml              ← uv 管理依赖
├── README.md                   ← 本文件
├── .env.example
├── main.py                     ← 入口：asyncio 调度多源并发
├── domain/                     ← 业务逻辑（零外部依赖）
│   ├── fetchers/               按源类型抓取
│   │   ├── rss_fetcher.py
│   │   ├── html_fetcher.py
│   │   └── telegram_fetcher.py   (V4 加)
│   ├── parsers/                结构化提取
│   ├── cleaners/               归一化 + 关键词过滤
│   └── deduper.py              SimHash 跨源去重
├── adapters/                   ← 外部 IO
│   ├── proxy.py                ⭐ 配置驱动是否走香港代理
│   ├── redis_writer.py         XADD news_stream + write:sentiment
│   └── source_registry.py      ⭐ 动态 RSS 源管理（M8 加）
├── config/                     ⭐ 数据源配置（YAML）
│   ├── domestic_sources.yaml
│   ├── overseas_sources.yaml
│   └── social_sources.yaml
├── api/                        ← FastAPI
│   ├── healthz.py                /healthz
│   └── metrics.py                /metrics
└── tests/{unit, integration}/
```

---

## 配置驱动数据源（核心设计）

### `config/domestic_sources.yaml` 示例

```yaml
sources:
  - name: 财联社化工
    type: rss
    url: https://www.cls.cn/telegraph/化工/rss
    interval: 300                 # 5 分钟一次
    use_proxy: false              # 国内源不走代理
    keywords:
      - 甲醇
      - MA
      - 原油
      - 煤炭
      - 港口
      - 库存
    blacklist:
      - 广告
      - 推广
    enabled: true
```

### `config/overseas_sources.yaml` 示例

```yaml
sources:
  - name: OilPrice
    type: rss
    url: https://oilprice.com/rss
    interval: 600
    use_proxy: true               # 海外源必须走代理
    proxy_addr: hk_proxy:1080
    keywords:
      - methanol
      - oil
      - crude
    enabled: true                 # P3 才设为 true
```

### 动态 RSS 新增（M8 阶段）

WebUI 上能 CRUD 数据源，**只在非交易时段允许**（交易时段守卫）。
配置存 Redis `rss_sources:<id>` + InfluxDB 历史。

---

## 对外接口

### 暴露的端点

| 类型 | 端口 | 路径 | 用途 |
|---|---|---|---|
| HTTP | 5000 | `/healthz` | 健康检查（含各源最近抓取时间） |
| HTTP | 5000 | `/metrics` | Prometheus 指标 |

### 写入的 Redis

| 目的 | Stream/Key | 频率 | 内容 |
|---|---|---|---|
| 业务流 | `news_stream` (Stream) | 5min/源 | 清洗后的新闻 IntelEvent |
| 持久化流 | `write:sentiment` (Stream) | 同上 | 含 raw + parsed + cleaned 三层 |
| 服务注册 | `services:intel_collector` (KV) | 10 秒续约 | ServiceInfo 30s TTL |
| 动态源管理 | `rss_sources:<id>` (KV) | 用户改时 | RssSource 配置（M8 起用） |

---

## 依赖

| 依赖 | 类型 | 必需 |
|---|---|---|
| Redis 7.2 | 内部服务 | ✅ |
| 各数据源 RSS / HTML 端点 | 外部 | ✅ |
| 香港代理 ECS | 外部 | 🟡 P3 才需要 |
| `contracts/intel_event.proto` | 内部契约 | ✅ |
| `contracts/rss_source.proto` | 内部契约 | ✅ |

---

## 配置（关键环境变量）

见 `.env.example`：

```bash
SERVICE_NAME=intel_collector
HEALTH_PORT=5000

REDIS_ADDR=redis:6379

# 香港代理（P3 启用海外源时配置）
HK_PROXY_ADDR=                     # 空 = 不启用代理
# HK_PROXY_ADDR=socks5://hk_proxy_ecs:1080

# 配置文件路径
CONFIG_DIR=/app/config

# 抓取并发数
FETCH_CONCURRENCY=5

LOG_LEVEL=info
```

---

## 启动方式

```bash
# 本地开发
cd services/intel_collector/
uv sync
python main.py

# Docker
docker compose --profile ai-only up -d intel_collector
```

---

## 测试

```bash
# Unit 测试（无网络依赖）
pytest tests/unit/

# Integration 测试（用 mock RSS 服务器）
pytest tests/integration/
```

测试 fetchers / parsers / cleaners 的纯逻辑，**不真发网络请求**。

---

## 已知限制 / 风险点

### 1. ⚠️ 数据源稳定性

RSS 源会偶尔失效（HTTP 500 / 解析失败）。
单源失败**不能影响**其他源——每个源跑在独立 asyncio task。

### 2. ⚠️ 海外源依赖香港代理

P3 阶段必须先购买香港 ECS 1C1G + v2ray/squid 才能跑。
代理挂了 → 海外源停摆，但**不影响国内源和交易主链路**。

### 3. ⚠️ 关键词过滤不够智能

V1 仅做关键词匹配，可能漏掉：
- 同义表达（"煤制烯烃" / "MTO"）
- 暗示性内容

V2 计划加 LLM 实体识别（在 llm_scorer 那边做，不在这里）。

### 4. ⚠️ Telegram 反爬

Telegram 公开群消息可读，但**频繁拉历史消息可能被封号**。
解决：
- 仅监听新消息（不主动拉历史）
- 用 user account 而非 bot account（bot 看不到群消息历史）

### 5. ⚠️ SimHash 去重边界情况

跨源同一新闻可能因排版差异 SimHash 距离 > 3 而漏判。
偶尔重复进 Stream **不致命**（llm_scorer 会发现 cleaned_id 已存在跳过）。

---

## 与 ARCHITECTURE.md 的关联

- **机制 5**：舆情监控三层数据源体系
- **机制 6**：5 阶段清洗管道（本服务负责 Stage 1-3）

---

## AI Agent 准则

- ✅ 加新数据源 → 改 YAML 配置文件，**不要**改代码
- ✅ 改清洗规则（如新关键词）→ unit 测试覆盖
- ❌ 不要在本服务调 LLM（那是 llm_scorer 的事）
- ❌ 不要在 `domain/` 里 import requests / httpx / redis
- ❌ 不要写 InfluxDB（必须走 db_writer）

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版（v2.1 合并方案） |
