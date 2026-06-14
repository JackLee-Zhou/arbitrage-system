# services/dashboard_web — 前端 Dashboard

> 本服务属于 arbitrage-system 微服务集合的一员。
> 在动手前必读：[`/ENGINEERING_GUIDE.md`](../../ENGINEERING_GUIDE.md) + [`/docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md)
> 重点章节：「Dashboard 技术栈」+「Dashboard 页面规划」

---

## 服务定位（一句话）

金融科技深色风 Web Dashboard，**调用 orchestrator REST API + WebSocket** 显示交易系统全景。**纯展示**，不直接连 Redis / InfluxDB / trading_agent。

---

## 来源：基于 Vision UI Dashboard React 改造

本项目基于 [Vision UI Dashboard React](https://github.com/creativetimofficial/vision-ui-dashboard-react)（Creative Tim，MIT License）改造。

⚠️ **必须保留**原 LICENSE.md 中的 Creative Tim 版权声明（见 `LICENSE_VISION_UI.md`）。

---

## 技术栈

| 工具 | 版本 | 来源 | 用途 |
|---|---|---|---|
| **React** | 17 | Vision UI | UI 框架（不主动升级 18） |
| **JavaScript** | ES2022 | Vision UI | 主语言（不强加 TypeScript） |
| **Create React App** | 5 | Vision UI | 脚手架（不主动迁移 Vite） |
| **Material-UI v5** | 5+ | Vision UI | 通用组件 + 容器 |
| **Emotion** | 11+ | MUI 默认 | CSS-in-JS |
| **React Router v5** | 5+ | Vision UI | 路由 |
| **Tremor** | 3+ | ⭐ 新增 | 金融图表（折线 / KPI 圆环 / 进度条） |
| **TradingView Lightweight Charts** | 4+ | ⭐ 新增 | K 线图 / 分时图（专业级） |
| **ApexCharts** | 3+ | Vision UI 保留 | 漏斗 / 桑基 / 热力图 |
| **TailwindCSS** | 3+ | ⭐ 新增 | 仅作 Tremor 依赖装上 |
| **TanStack Query** | 5+ | ⭐ 新增 | API 数据管理 |
| **Zustand** | 4+ | ⭐ 新增 | 全局状态 |
| **socket.io-client / 原生 WS** | - | ⭐ 新增 | 实时推送 |
| **Monaco Editor** | latest | ⭐ 新增（M10） | YAML 策略编辑器 |
| **nginx** | alpine | ⭐ 新增 | 生产托管静态资源 |

---

## 图表库分工（重要）

| 图表类型 | 用什么 | 理由 |
|---|---|---|
| K 线图、分时图 | **TradingView Lightweight Charts** | 行业标杆，性能最好 |
| 趋势折线、面积图、KPI 圆环 | **Tremor** | 金融科技深色风开箱即用 |
| 漏斗图、桑基图、热力图 | ApexCharts（Vision UI 自带） | Tremor 不支持，用模板已有 |
| 简单表格 / 数字卡片 | MUI 原生 | 与 Vision UI 风格统一 |

⭐ **容器（Card / Box / Typography）始终用 MUI**，**只把图表替换成 Tremor 或 TradingView**：

```jsx
import { Card } from "@mui/material";          // 容器 = MUI
import { AreaChart } from "@tremor/react";      // 图表 = Tremor

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
```

---

## 内部结构

```
dashboard_web/
├── Dockerfile                    ← 多阶段构建：Node builder + nginx 托管
├── package.json                  ← 改 name 为 arbitrage-dashboard
├── README.md                     ← 本文件
├── LICENSE_VISION_UI.md          ⚠️ 必须保留 Creative Tim MIT 协议
├── nginx.conf                    生产 nginx 配置
├── public/                       Vision UI 自带：字体、图标
├── src/
│   ├── assets/                   Vision UI 自带：字体 / 图片
│   ├── components/               Vision UI 自带的基础组件
│   │   ├── Cards/
│   │   ├── Tables/
│   │   ├── Sidenav/                侧边栏（保留改造）
│   │   └── Configurator/           主题配置面板（保留）
│   ├── layouts/                  ⭐ 业务页面（按 P1/P2/P3 渐进）
│   │   ├── dashboard/              概览页
│   │   ├── futures/
│   │   │   ├── positions/
│   │   │   ├── orders/
│   │   │   ├── trades/
│   │   │   ├── intraday/             P2: TradingView 分时图
│   │   │   ├── kline/                P2: TradingView K 线
│   │   │   └── statement/            P3: 对账单
│   │   ├── ai/
│   │   │   ├── scores/               AI 评分实时
│   │   │   └── history/              AI 评分历史曲线
│   │   ├── nodes/                  节点状态详情
│   │   ├── host/                   主机性能
│   │   ├── strategies/             P3: 策略管理（Monaco YAML）
│   │   ├── risk/                   风控参数
│   │   ├── audit/                  审计日志 + 决策溯源
│   │   └── sentiment/              P3: 舆情看板
│   ├── examples/                 Vision UI 自带（参考用，不全删）
│   ├── api/                      ⭐ orchestrator 客户端
│   │   ├── client.js               axios 封装
│   │   ├── ws.js                   WebSocket 连接管理
│   │   └── queries/                TanStack Query hooks
│   │       ├── useOverview.js
│   │       ├── usePositions.js
│   │       ├── useAIScore.js
│   │       └── ...
│   ├── stores/                   ⭐ Zustand 全局状态
│   │   ├── system.js               系统状态
│   │   └── trading.js              交易状态
│   ├── charts/                   ⭐ 图表组件封装
│   │   ├── tremor/                 Tremor 图表封装
│   │   ├── tradingview/            TradingView K 线封装
│   │   └── apex/                   保留的 ApexCharts
│   ├── theme/                    Vision UI 自带主题（颜色已经够用）
│   ├── routes.js                 路由配置（按页面清单改）
│   ├── context/                  Vision UI 自带：UI 状态
│   ├── App.js
│   └── index.js
└── tests/
    ├── e2e/                      Playwright 端到端
    └── unit/                     Jest（Vision UI 自带）
```

---

## 信息架构与导航设计（v1.1 新增）

### 顶部菜单：5 个 tab

```
┌─────────────────────────────────────────────────────────┐
│ 🏠 概览  📊 交易  🤖 AI  ⚙️ 系统  🔧 管理              │
└─────────────────────────────────────────────────────────┘
```

每个 tab 下的二级页面：

| Tab | 二级菜单 | 路由 |
|---|---|---|
| 🏠 **概览** | 三区汇总首页 | `/` |
| 📊 **交易** | 📈 期货 / 📉 股票（占位）| `/futures/*` `/stocks/*` |
| 🤖 **AI** | AI 评分 / 评分历史 / 舆情看板 / 决策溯源 | `/ai/*` |
| ⚙️ **系统** | 节点状态 / 主机性能 / 审计日志 | `/system/*` |
| 🔧 **管理** | 策略管理 / 风控参数 / 数据源管理 | `/manage/*` |

#### 📊 交易 tab 的二级菜单（v1.2 新增）

「交易」tab 进入后，左侧二级菜单先选 **期货** 或 **股票**：

```
┌──────────────────────────────────────────────────────┐
│ 📊 交易                                               │
│  ├─ 📈 期货 (default)                                │
│  │   ├─ 合约浏览（含详情大卡片）  /futures/browse     │
│  │   ├─ 委托                       /futures/orders    │
│  │   ├─ 成交                       /futures/trades    │
│  │   └─ 对账单                     /futures/statement │
│  └─ 📉 股票（V2 占位）                                │
│      └─ "股票模块暂未开放"         /stocks            │
└──────────────────────────────────────────────────────┘
```

⚠️ V1 阶段「股票」点击后只显示**占位提示页**（"📉 股票模块 - 状态：规划中（V2 阶段实施）- 计划接入 QMT/easytrader - 预计上线 P3 完成后"）。

### 首页设计：三区汇总（核心决策）

首页（`/`）不是某个具体场景的视图，而是**三区汇总**——让用户按当前需要点击进详情：

```
┌──────────────────────────────────────────────────────────────┐
│ 🏠 首页                                                       │
│                                                              │
│ ┌────────────────────────┐  ┌──────────────────────────────┐  │
│ │ 📊 盯盘区（盘中关注）    │  │ 🤖 AI 区                      │  │
│ │ ─────────────────────  │  │ ─────────────────────────  │  │
│ │ • 当前价格 + 涨跌幅     │  │ • Live AI 评分 + 老化时间    │  │
│ │ • 当前持仓 + 浮动盈亏   │  │ • 最近紧急触发事件            │  │
│ │ • 今日委托 / 成交次数  │  │ • 舆情指数（P3）              │  │
│ │ → 点击进入【交易】tab   │  │ → 点击进入【AI】tab           │  │
│ └────────────────────────┘  └──────────────────────────────┘  │
│                                                              │
│ ┌────────────────────────┐  ┌──────────────────────────────┐  │
│ │ ⚙️ 系统区               │  │ 📰 最近事件流                 │  │
│ │ ─────────────────────  │  │ ─────────────────────────  │  │
│ │ • 9 个服务健康灯         │  │ • 实时滚动（订单成交/告警/  │  │
│ │ • 内存 / 磁盘使用率      │  │   AI 评分更新等）             │  │
│ │ • 最近告警计数            │  │ • 点击事件查看详情            │  │
│ │ → 点击进入【系统】tab    │  │                                │  │
│ └────────────────────────┘  └──────────────────────────────┘  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

⚠️ **首页不是单一场景**——三区并列，让用户**按需选择**当前关注什么。

### 为什么不做"按时段智能默认"

考虑过：交易时段跳盯盘视图、盘后跳复盘视图……
但**否决了**，理由：
1. 首页一定是 `/`，不应该有"今天打开和昨天打开看到不同首页"的认知负担
2. 三区汇总让用户自己选，反而最直观
3. 实施简单（不需要时段判断逻辑）

---

## 期货合约浏览页设计（v1.2 新增）

### 总体布局：左列表 + 右详情大卡片

```
┌─────────────────────────────────────────────────────────────────────┐
│ 📈 期货 / 合约浏览                                  💰 持仓: MA2509 + 1 手 +¥234 │
├──────────────────┬──────────────────────────────────────────────────┤
│                  │                                                  │
│  [🔍 搜索]       │   MA2509 甲醇主力     2350.0 ↑0.5% (+12)        │
│                  │   ───────────────────────────────────────       │
│  ⭐ 持仓中 (2)   │                                                  │
│  ─────────       │   ┌──────────────────────────┬──────────────┐ │
│  ⭐ MA2509 持 1  │   │  🕐 分时图（左上）        │  📋 盘口五档 │ │
│  ⭐ FU2509 持 1  │   │   实时 1Hz 推送           │   实时       │ │
│  ─────────       │   │                            │              │ │
│                  │   ├──────────────────────────┤   卖5 2354 12│ │
│  全部合约 (327) │   │  📊 日 K 线（左下）        │   卖4 2353 18│ │
│  ─────────       │   │   TradingView Charts      │   卖3 2352 25│ │
│  AG2509 沪银      │   │                            │   卖2 2351 30│ │
│  AL2509 沪铝      │   │                            │   卖1 2350 45│ │
│  AP2509 苹果      │   │                            │   ────────  │ │
│  AU2509 沪金      │   │                            │   买1 2349 50│ │
│  ...             │   │                            │   买2 2348 32│ │
│  CU2509 沪铜      │   │                            │   买3 2347 28│ │
│  ...             │   │                            │   买4 2346 20│ │
│  MA2509 甲醇 ⭐   │   └──────────────────────────┴──────────────┘ │
│  ...             │                                                  │
│                  │   💼 我的持仓（如有）                            │
│                  │   ─────────────────────────                     │
│                  │   方向: 多      数量: 1 手                       │
│                  │   开仓价: 2342  当前价: 2350                     │
│                  │   浮动盈亏: +¥80    收益率: +0.34%               │
│                  │                                                  │
│                  │   [🛑 紧急平仓本合约]                            │
└──────────────────┴──────────────────────────────────────────────────┘
```

### 关键设计点

#### 1. 合约列表（左栏）

- **持仓中合约置顶**（带 ⭐ + 持仓手数 + 浮动盈亏标记）
- **搜索框**：直接输入合约代码模糊匹配（如 "MA"、"2509"、"甲醇"）
- 搜索为空时显示**全部 ~330 个合约**（按字母 + 合约月排序）
- 列表每行显示：**代码 + 中文名 + 最新价 + 涨跌幅**
- **不做品类 tab 过滤**（仅靠搜索 + 持仓置顶，简单实用）
- 列表实时刷新（依赖 OPT-B 后端推送的 last_price）

#### 2. 详情大卡片（右栏）

只有点击列表某个合约后才展示。三栏内同屏布局（A 选项）：

| 区域 | 位置 | 内容 |
|---|---|---|
| **顶栏** | 全宽 | 合约代码 + 中文名 + 最新价 + 涨跌幅（实时跳动） |
| **左上**（约 60% 宽 × 50% 高）| 分时图 | TradingView Lightweight Charts，1 秒级聚合 |
| **左下**（同上）| 日 K 线 | TradingView Charts，可切周期（日/周） |
| **右栏**（约 40% 宽）| 盘口五档 | 五档买卖盘 + 实时跳动（红绿配色） |
| **右下**（持仓时显示）| 我的持仓 | 方向 / 数量 / 开仓价 / 浮动盈亏 / 紧急平仓按钮 |

#### 3. 全局持仓固定显示（顶栏右侧）

只在 **📊 交易 tab** 内显示（不全局）：

```
顶部导航条右侧固定：
┌──────────────────────────────┐
│ 💰 持仓: MA2509 +1  +¥234    │  ← 多个持仓时显示总结 / 最大单
└──────────────────────────────┘
```

具体格式：
- **单合约持仓**：`💰 持仓: MA2509 +1 +¥234`
- **多合约持仓**：`💰 总持仓 2 个合约 总浮动 +¥123`（点击展开详情）
- **无持仓**：`💰 当前无持仓`

⚠️ 仅在 `/futures/*` 路由下显示。其他 tab（AI / 系统 / 管理）不显示——保持顶部空间。

### 数据来源（依赖后端 OPT-A / OPT-B / OPT-C）

| 数据 | 来源 | 推送方式 |
|---|---|---|
| 合约元数据（330 个） | Redis `instruments:all`（OPT-A） | 启动时一次性 GET |
| 列表实时最新价 | Redis `last_price:<symbol>`（OPT-B） | WebSocket `prices` 频道 |
| 详情卡片分时图 | InfluxDB `ma_tick` 1 分钟聚合 | HTTP GET（首屏）+ WS 增量 |
| 详情卡片日 K 线 | InfluxDB `ma_1min` 按天聚合 | HTTP GET 一次 |
| 详情卡片盘口五档 | Redis `tick:<symbol>` Stream 末位 | WebSocket `contract:<symbol>` 频道 |
| 持仓信息 | trading_agent gRPC | WebSocket `positions` 频道 |
| 主力合约标识 | `instruments:all` 中的 `is_main` 字段（OPT-C） | 同元数据 |

### 实现的 3 个后端依赖（必须先做）

详见 `OPTIMIZATIONS.md`：

- ⚠️ **OPT-A**：ctp_tick 启动时写合约元数据（330 个）到 Redis + InfluxDB
- ⚠️ **OPT-B**：ctp_tick 维护「全市场最新价快照」到 Redis（1s 节流）
- ⚠️ **OPT-C**：orchestrator 计算并标记主力合约（`is_main`）

如果以上 3 个未完成，dashboard 合约浏览页**只能展示 SUBSCRIBE_SYMBOLS 中的几个合约**。

---

## 紧急平仓位置（⚠️ 重要决策）

### 决策：仅在「合约浏览」详情卡片中有紧急平仓按钮

**不**在顶部固定全局按钮。
**只有点击进具体合约后**，详情卡片右下角"我的持仓"区显示紧急平仓按钮（持仓时才显示）。

### ⚠️ 已知风险（必须显式记录）

这个决策**有真实的资金安全风险**：
- 极端行情时（比如 2 秒内 ±2% 波动），从其他页面切到持仓页平仓需要 4-5 步操作
- 比工业界主流交易终端（博弈大师 / TradeStation / Bloomberg）的做法保守
- 如果将来发生"因 UI 操作步骤多而错失平仓时机"，请回查本决策

### 折中实现（部分缓解）

虽然不放全局按钮，但要做以下加固：

1. **持仓页平仓按钮必须醒目**（红色 + 大尺寸 + 不被滚动条遮挡）
2. **按钮二次确认弹窗**：弹窗里有【仅平仓本合约】和【全部平仓 + 暂停策略】两个选项
3. **实盘环境**额外要求输入「确认」二字才执行
4. **钉钉告警时附带快捷链接**：直接跳到持仓页（不用先点导航）
   ```
   钉钉消息中：[🔗 立即查看持仓](http://ali-trade:8000/futures/positions)
   ```
5. **键盘快捷键**：在任意页面按 `Esc + Esc` 可跳转到持仓页（**不是直接平仓**，只是跳转）

### 未来重新评估的触发条件

如果出现以下情况，重新讨论这个决策：
- 实际使用中遇到「找不到平仓按钮」的紧急时刻
- 黑天鹅行情期间因多步操作错失最佳平仓时机
- 用户主动反馈不便

---

## 实时数据：P1 就上 WebSocket

### 推送架构

```
浏览器 dashboard_web
    ↑ WebSocket /ws
    │
orchestrator (后端)
    ↑ Redis Pub/Sub + gRPC stream
    │
trading_agent / 各服务
```

### 三种消息类型（同 ARCHITECTURE.md）

| 消息类型 | 触发时机 | 频率 |
|---|---|---|
| `snapshot` | 客户端首次连接 | 一次性全量 |
| `delta` | 状态变化 | 1Hz 节流（防卡） |
| `event` | 关键事件（订单/告警/紧急） | 实时（不节流） |

### 前端订阅频道

```typescript
// 默认订阅
ws.send({
  action: "subscribe",
  channels: ["system_status", "positions", "scores", "alerts"]
});

// 切到分时图时
ws.send({
  action: "subscribe",
  channels: [..., "ticks"]   // 加 tick 推送
});

// 切走分时图时
ws.send({
  action: "unsubscribe",
  channels: ["ticks"]         // 取消 tick 推送
});
```

### WebSocket 兜底机制

- 断线 → 指数退避重连（1s / 2s / 4s / 8s ... 最多 30s）
- 重连成功 → **必须先取 snapshot**（不能直接消费 delta）
- 60 秒无消息 → 主动 ping，无响应则重连
- 长时间断线 → 红色告警提示用户「实时数据已断开」

---

## 视觉风格：保留 Vision UI

### 决策：完全保留 Vision UI 的视觉风格

- 玻璃拟态（glassmorphism）
- 紫蓝渐变 + 光晕效果
- 暗色背景

### 但金融交易特有元素必须自己写或调整

| Vision UI 自带 | 金融交易需要 | 处理方式 |
|---|---|---|
| 销售额折线图 | 价格分时图（清晰锐利） | TradingView Lightweight Charts 替代 |
| KPI 数字卡片 | 持仓 PnL（红绿+实时跳动） | ✅ 直接复用，加配色（涨绿跌红） |
| 用户列表表格 | 委托/成交流水（高密度） | ⚠️ 表格密度调整 |
| 销售漏斗 | AI 评分圆环（带光晕） | ✅ Vision UI 自带 |
| Toast 通知 | 实时事件流（持续滚动） | ⚠️ 自己写组件 |
| 一般按钮 | 紧急平仓按钮（醒目+二次确认） | ⚠️ 自己写 |

### 配色补充约定（Vision UI 之外）

| 业务语义 | 颜色 | 用途 |
|---|---|---|
| 涨 / 盈利 / 健康 | `#00FF94`（青绿） | 价格上涨、PnL 正、服务在线 |
| 跌 / 亏损 / 异常 | `#FF4D6D`（粉红） | 价格下跌、PnL 负、服务挂了 |
| AI 评分中性 | `#9D5FFF`（紫色） | shadow 评分、未老化警告 |
| AI 评分老化 | `#FFAB00`（橙色） | 评分超过 15 分钟 |
| 紧急 | `#FF0033`（亮红） | 紧急平仓按钮、P0 告警 |

---

## 手机端：不支持（L0）

- ✅ 桌面浏览器优化（1920×1080 / 2560×1440 / Retina 都要好看）
- ❌ 移动端不做响应式
- ❌ 平板（iPad）不做响应式

如果在手机访问 → 显示提示「请在桌面浏览器访问」+ 仍然显示一个**简化版"紧急平仓"页**作为兜底。

---

## 页面清单（按 P1/P2/P3 渐进）

按 5 tab 架构组织（v1.1 调整）：

### 🏠 概览 tab
| 页面 | 路由 | P1 | P2 | P3 | 备注 |
|---|---|---|---|---|---|
| 三区汇总首页 | `/` | ✅ | ✅ | ✅ | 盯盘区 + AI 区 + 系统区 + 事件流 |

### 📊 交易 tab（v1.2 大改）

| 页面 | 路由 | P1 | P2 | P3 | 备注 |
|---|---|---|---|---|---|
| **期货合约浏览** | `/futures/browse` | ✅ | ✅ | ✅ | ⭐ **核心页面**：左列表 + 右详情卡片（分时/日K/盘口五档/持仓） |
| 期货 - 委托 | `/futures/orders` | ✅ | ✅ | ✅ | 当日委托 + 历史筛选 |
| 期货 - 成交 | `/futures/trades` | ✅ | ✅ | ✅ | 当日成交明细 |
| 期货 - 对账单 | `/futures/statement` | ❌ | 简版 | ✅ | 按日 / 按月汇总 |
| **股票（占位页）** | `/stocks` | ✅ | ✅ | ✅ | 显示"暂未开放，V2 阶段实施" |

⚠️ v1.1 中独立的「持仓 / 分时图 / K 线」页**已合并入「合约浏览」详情卡片**。
⚠️ 紧急平仓按钮在「合约浏览」详情卡片右下角（持仓时才显示）。
⚠️ 顶栏右侧持仓固定栏只在 `/futures/*` 路由下显示。

### 🤖 AI tab
| 页面 | 路由 | P1 | P2 | P3 | 备注 |
|---|---|---|---|---|---|
| AI 评分 | `/ai/scores` | ✅ | ✅ | ✅ | live + shadow + 老化时间 |
| 评分历史 | `/ai/scores/history` | ✅ | ✅ | ✅ | 历史曲线（Tremor） |
| 舆情看板 | `/ai/sentiment` | ❌ | ❌ | ✅ | 新闻流 + 情感分布 |
| 决策溯源 | `/ai/decisions/[id]` | ❌ | ❌ | ✅ | 6 层 ID 反查（机制 6） |

### ⚙️ 系统 tab
| 页面 | 路由 | P1 | P2 | P3 | 备注 |
|---|---|---|---|---|---|
| 节点状态 | `/system/nodes` | ✅ | ✅ | ✅ | 9 个服务健康度 |
| 主机性能 | `/system/host` | ✅ | ✅ | ✅ | CPU/内存/磁盘/网络 |
| 审计日志 | `/system/audit` | ✅ 简版 | ✅ | ✅ | 所有修改型操作 |

### 🔧 管理 tab
| 页面 | 路由 | P1 | P2 | P3 | 备注 |
|---|---|---|---|---|---|
| 策略管理 | `/manage/strategies` | 只读 | 启停 | ✅ 完整 | Monaco YAML 编辑（M10） |
| 风控参数 | `/manage/risk` | 只读 | ✅ | ✅ | 修改要走时段守卫 |
| 数据源管理 | `/manage/sources` | ❌ | ✅ | ✅ | RSS 动态新增（M8） |

### 全局元素
| 元素 | 位置 | P1 | 备注 |
|---|---|---|---|
| 顶部 5 个 tab | 顶部导航 | ✅ | 概览 / 交易 / AI / 系统 / 管理 |
| 紧急平仓按钮 | 仅 `/futures/browse` 详情卡片右下（持仓时显示） | ✅ | ⚠️ 见「紧急平仓位置」章节 |
| 持仓固定栏 | 顶部右侧（仅 `/futures/*` 路由下显示） | ✅ | 显示当前持仓 + 浮动盈亏 |
| 实时事件流（右下角） | 全局浮动 | ✅ | WebSocket 推送的告警 / 订单成交事件 |
| 钉钉快捷链接 | 钉钉消息中 | ✅ | 直跳合约浏览详情卡片 |

---


| 页面 | 路由 | P1 | P2 | P3 |
|---|---|---|---|---|
| 概览 | `/` | ✅ 高级 | ✅ | ✅ |
| 节点状态 | `/nodes` | ✅ | ✅ | ✅ |
| 主机性能 | `/host` | ✅ | ✅ | ✅ |
| 期货 - 持仓 | `/futures/positions` | ✅ | ✅ | ✅ |
| 期货 - 委托 | `/futures/orders` | ✅ | ✅ | ✅ |
| 期货 - 成交 | `/futures/trades` | ✅ | ✅ | ✅ |
| 期货 - 分时图 | `/futures/intraday` | ❌ | ✅ | ✅ |
| 期货 - K 线 | `/futures/kline` | ❌ | ✅ | ✅ |
| 期货 - 对账单 | `/futures/statement` | ❌ | 简版 | ✅ |
| AI 评分 | `/ai/scores` | ✅ | ✅ | ✅ |
| AI 评分历史 | `/ai/scores/history` | ✅ | ✅ | ✅ |
| 舆情看板 | `/sentiment` | ❌ | ❌ | ✅ |
| 决策溯源 | `/audit/decisions/[id]` | ❌ | ❌ | ✅ |
| 策略管理 | `/strategies` | 只读 | 启停 | ✅ 完整 |
| 风控参数 | `/risk` | 只读 | ✅ | ✅ |
| 审计日志 | `/audit` | ✅ 简版 | ✅ | ✅ |
| 紧急平仓 | 全局按钮 | ✅ | ✅ | ✅ |

---

## 数据流

```
浏览器 / 手机
    │
    │ HTTP GET（首屏加载）
    ▼
TanStack Query（缓存 + 重试）
    │
    │ axios → orchestrator REST API
    ▼
渲染 React 组件 + 显示

并行：
    │
    │ WebSocket 长连接
    ▼
ws.js（自动重连 + snapshot 兜底）
    │
    │ Zustand store 更新
    ▼
React 组件订阅 store → 自动重渲染
```

---

## 对外接口

### 暴露的端口

| 类型 | 端口 | 路径 | 用途 |
|---|---|---|---|
| HTTP | 8000 | `/` | nginx 托管静态资源 |
| HTTP | 8000 | `/healthz` | nginx 静态检查端点 |

### 不主动连任何后端服务
（除了 orchestrator 的 API + WebSocket）

⚠️ **不允许**：
- 直接连 Redis
- 直接连 InfluxDB
- 直接调 trading_agent gRPC

---

## 启动方式

### 本地开发

```bash
cd services/dashboard_web/
npm install
npm start
# 浏览器自动打开 :3000
# 默认连本机的 orchestrator（环境变量 REACT_APP_API_URL）
```

### 生产构建

```bash
npm run build      # 生成 build/ 静态资源

# Docker
docker compose --profile ui up -d dashboard_web
# 浏览器访问 http://ali-trade:8000
```

---

## 测试

```bash
# Unit 测试（Jest，Vision UI 自带）
npm test

# E2E 测试（Playwright，假设 orchestrator 已起来）
npm run test:e2e
```

---

## 已知限制 / 风险点

### 1. ⚠️ Vision UI 维护活跃度低
Creative Tim 这个仓库最近 2 年更新少。
**好处**：稳定（坑都被踩平了）
**风险**：依赖安全更新慢，自己关注 npm audit

### 2. ⚠️ React 17（不是 18）
Vision UI 用的是 React 17，**不要主动升级**到 18，会有兼容性踩坑。

### 3. ⚠️ JavaScript（不是 TypeScript）
保持原项目风格，不主动迁移 TypeScript。

### 4. ⚠️ MIT License 要求
保留 `LICENSE_VISION_UI.md` 文件 + README 致谢。

### 5. ⚠️ WebSocket 断线
浏览器在网络抖动时 WebSocket 会断，必须有自动重连 + snapshot 对账。

### 6. ⚠️ 移动端适配
Vision UI 默认对移动端不太友好，需要测试手机浏览器访问。

---

## 与 ARCHITECTURE.md 的关联

- 「Dashboard 技术栈（v2.3 — 基于 Vision UI 模板）」
- 「Dashboard 页面规划」16 个页面渐进交付

---

## AI Agent 准则

### 通用准则
- ✅ 加新页面 → 在 routes.js 注册 + 在本文档页面清单更新
- ✅ 改 API 调用 → 同步 orchestrator 端的 REST handler
- ❌ 不要在前端做业务决策（"如果价格 > X 显示警告" → 这种判断应该在后端）
- ❌ 不要直连 Redis / InfluxDB / trading_agent gRPC
- ❌ 不要主动升级 React 18 / 迁移 TypeScript / 换 Next.js
- ❌ 不要删除 LICENSE_VISION_UI.md

### ⭐ 接口缺口处理（重要！）

**写代码时如果发现 orchestrator 缺接口，绝不要直接写死 mock 数据凑合**。必须按以下流程：

```
1. 检查 services/orchestrator/README.md 「REST API」表是否已存在该接口
   ↓
   ├─ 存在 → 直接调用，OK
   └─ 不存在 → 走以下流程
   ↓
2. 检查 services/orchestrator/API_REQUESTS.md 是否已登记该需求
   ↓
   ├─ 已登记且状态 [x] 已实现 → 提醒用户：orchestrator README 未同步
   ├─ 已登记且状态 [ ]/[~] → 在自己代码里加 TODO 注释指向 REQ-XXX，先用 mock 推进
   └─ 未登记 → 在 API_REQUESTS.md 登记一条新 REQ-XXX，然后加 TODO + mock
   ↓
3. 代码注释规范：
      // TODO(REQ-001): 等 orchestrator 实现 GET /positions/{symbol} 后替换 mock
      const position = await fetchMockPosition(symbol);
   ↓
4. 任务「完成报告」中明确列出本次任务发起的所有 REQ-XXX
```

**接入闭环**（看到 orchestrator 已实现的 REQ）：

```
1. 开新 dashboard 任务时，第一件事就是读 API_REQUESTS.md
   ↓
2. 找到所有 [x] 已实现但未 [✓] 接入的 REQ-XXX
   ↓
3. 把对应代码里的 TODO 替换为真实调用
   ↓
4. 跑测试 + /code-review
   ↓
5. 状态改为 [✓] 已接入，记录接入时间
```

详细规则见 `services/orchestrator/API_REQUESTS.md`。

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版（v2.3 基于 Vision UI 模板） |
| v1.1 | 2026-06-14 | 大幅细化设计：(1) 新增「信息架构与导航设计」章节定义 5 tab 顶部菜单（概览/交易/AI/系统/管理）+ 三区汇总首页；(2) 新增「紧急平仓位置」决策章节（仅在持仓页有，含 5 项部分缓解措施 + 风险显式记录）；(3) 新增「实时数据：P1 就上 WebSocket」三种消息类型；(4) 视觉风格保留 Vision UI（玻璃拟态 + 紫蓝渐变），定义业务语义配色；(5) 手机端不支持（L0），仅桌面浏览器；(6) 页面清单按 5 tab 重组路由（如 `/futures/*` `/ai/*` `/system/*` `/manage/*`） |
| v1.2 | 2026-06-14 | 期货 / 股票分流：交易 tab 拆为「📈 期货」+「📉 股票（V2 占位）」；新增「期货合约浏览页设计」章节，定义左列表 + 右详情大卡片（分时/日K/盘口五档/持仓）三栏布局；持仓 + 浮动盈亏在 `/futures/*` 路由下顶栏右侧固定显示；紧急平仓按钮迁移到合约浏览详情卡片中（持仓时显示）；列表设计为「仅搜索 + 持仓置顶」（最简方案）；主力合约计算放 orchestrator；新增 3 个后端依赖 OPT-A/OPT-B/OPT-C |
| v1.3 | 2026-06-14 | AI Agent 准则加「接口缺口处理」流程：发现缺接口必须在 `services/orchestrator/API_REQUESTS.md` 登记 REQ-XXX + TODO 注释指向，禁止直接写 mock；接入闭环（[x] → [✓]）流程化 |
