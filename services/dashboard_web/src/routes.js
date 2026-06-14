// ============================================================================
// Dashboard Routes — 5-tab structure
// ============================================================================

// ---- Tab: 概览 -------------------------------------------------------------
import Overview from "layouts/dashboard";

// ---- Tab: 交易 -------------------------------------------------------------
import FuturesBrowse from "layouts/futures/Browse";
import FuturesOrders from "layouts/futures/Orders";
import FuturesTrades from "layouts/futures/Trades";
import StocksPlaceholder from "layouts/stocks/Placeholder";

// ---- Tab: AI ---------------------------------------------------------------
import AIScores from "layouts/ai/Scores";
import AIScoresHistory from "layouts/ai/ScoresHistory";
import AISentiment from "layouts/ai/Sentiment";
import AIDecisions from "layouts/ai/Decisions";

// ---- Tab: 系统 ------------------------------------------------------------
import SystemNodes from "layouts/system/Nodes";
import SystemHost from "layouts/system/Host";
import SystemAudit from "layouts/system/Audit";

// ---- Tab: 管理 ------------------------------------------------------------
import ManageStrategies from "layouts/manage/Strategies";
import ManageRisk from "layouts/manage/Risk";
import ManageSources from "layouts/manage/Sources";

// ---- Icons ----------------------------------------------------------------
import { IoHome } from "react-icons/io5";
import { IoStatsChart } from "react-icons/io5";
import { IoHardwareChip } from "react-icons/io5";
import { IoSettings } from "react-icons/io5";
import { IoTrendingUp } from "react-icons/io5";

const routes = [
  // ---- 🏠 概览 tab ---------------------------------------------------------
  { type: "title", title: "🏠 概览", key: "overview-title" },
  {
    type: "collapse",
    name: "总览仪表盘",
    key: "overview",
    route: "/",
    icon: <IoHome size="20px" color="inherit" />,
    component: Overview,
    noCollapse: true,
  },

  // ---- 📊 交易 tab ---------------------------------------------------------
  { type: "title", title: "📊 交易", key: "trading-title" },
  {
    type: "collapse",
    name: "期货合约浏览",
    key: "futures-browse",
    route: "/futures/browse",
    icon: <IoTrendingUp size="20px" color="inherit" />,
    component: FuturesBrowse,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "期货委托",
    key: "futures-orders",
    route: "/futures/orders",
    icon: <IoStatsChart size="20px" color="inherit" />,
    component: FuturesOrders,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "期货成交",
    key: "futures-trades",
    route: "/futures/trades",
    icon: <IoStatsChart size="20px" color="inherit" />,
    component: FuturesTrades,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "股票",
    key: "stocks",
    route: "/stocks",
    icon: <IoTrendingUp size="20px" color="inherit" />,
    component: StocksPlaceholder,
    noCollapse: true,
  },

  // ---- 🤖 AI tab -----------------------------------------------------------
  { type: "title", title: "🤖 AI", key: "ai-title" },
  {
    type: "collapse",
    name: "AI 评分",
    key: "ai-scores",
    route: "/ai/scores",
    icon: <IoHardwareChip size="20px" color="inherit" />,
    component: AIScores,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "评分历史",
    key: "ai-scores-history",
    route: "/ai/scores/history",
    icon: <IoStatsChart size="20px" color="inherit" />,
    component: AIScoresHistory,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "舆情看板",
    key: "ai-sentiment",
    route: "/ai/sentiment",
    icon: <IoTrendingUp size="20px" color="inherit" />,
    component: AISentiment,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "决策溯源",
    key: "ai-decisions",
    route: "/ai/decisions/:id",
    icon: <IoStatsChart size="20px" color="inherit" />,
    component: AIDecisions,
    noCollapse: true,
  },

  // ---- ⚙️ 系统 tab ---------------------------------------------------------
  { type: "title", title: "⚙️ 系统", key: "system-title" },
  {
    type: "collapse",
    name: "节点状态",
    key: "system-nodes",
    route: "/system/nodes",
    icon: <IoHardwareChip size="20px" color="inherit" />,
    component: SystemNodes,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "主机性能",
    key: "system-host",
    route: "/system/host",
    icon: <IoStatsChart size="20px" color="inherit" />,
    component: SystemHost,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "审计日志",
    key: "system-audit",
    route: "/system/audit",
    icon: <IoStatsChart size="20px" color="inherit" />,
    component: SystemAudit,
    noCollapse: true,
  },

  // ---- 🔧 管理 tab ---------------------------------------------------------
  { type: "title", title: "🔧 管理", key: "manage-title" },
  {
    type: "collapse",
    name: "策略管理",
    key: "manage-strategies",
    route: "/manage/strategies",
    icon: <IoSettings size="20px" color="inherit" />,
    component: ManageStrategies,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "风控参数",
    key: "manage-risk",
    route: "/manage/risk",
    icon: <IoSettings size="20px" color="inherit" />,
    component: ManageRisk,
    noCollapse: true,
  },
  {
    type: "collapse",
    name: "数据源",
    key: "manage-sources",
    route: "/manage/sources",
    icon: <IoSettings size="20px" color="inherit" />,
    component: ManageSources,
    noCollapse: true,
  },
];

export default routes;
