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

## 页面清单（按 P1/P2/P3 渐进）

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

- ✅ 加新页面 → 在 routes.js 注册 + 在本文档页面清单更新
- ✅ 改 API 调用 → 同步 orchestrator 端的 REST handler
- ❌ 不要在前端做业务决策（"如果价格 > X 显示警告" → 这种判断应该在后端）
- ❌ 不要直连 Redis / InfluxDB / trading_agent gRPC
- ❌ 不要主动升级 React 18 / 迁移 TypeScript / 换 Next.js
- ❌ 不要删除 LICENSE_VISION_UI.md

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版（v2.3 基于 Vision UI 模板） |
