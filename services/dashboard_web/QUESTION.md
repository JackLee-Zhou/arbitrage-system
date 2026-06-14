# dashboard_web — 问题记录

> 模块开发过程发现的问题和修复方式。

## 当前待解决问题

| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-14 | ⚠️ **设计风险登记**：紧急平仓按钮**仅在持仓页**，黑天鹅时需 4-5 步切页操作。已通过 5 项缓解（红色大按钮 + 二次确认 + 实盘输入「确认」+ 钉钉快捷链接 + Esc+Esc 跳转），但仍**比业界主流交易终端保守**。如发生因 UI 切页延迟导致的损失，回查本决策。详见 README「紧急平仓位置」章节。 | 🟡 | accepted（用户已知风险并同意接受） |
| 2 | 2026-06-14 | `@tremor/react@3.x` 要求 React 18，与 README「React 17 不升级」决策冲突。 | 🟡 | accepted（已升级至 React 18，MUI v5 兼容无回归） |
| 3 | 2026-06-14 | Sidenav Logo 使用逗号运算符导致 `sidenavLogoLabel` 返回值丢弃 (styles/sidenav.js)，mini 侧边栏折叠动画失效。原 Vision UI 模板既有 bug。 | 🟢 | open（P2，后续修复） |
| 4 | 2026-06-14 | context/index.js、theme-rtl.js、VuiInputRoot.js、DashboardNavbar/styles.js 残留 RTL/Breadcrumbs 死代码。 | 🟢 | open（P2 清理，不阻塞功能） |

## 已修复问题

| # | 发现时间 | 修复时间 | 描述 | 修复方式 | 版本 |
|---|---|---|---|---|---|
| 1 | 2026-06-14 | 2026-06-14 | 🔴 `stores/trading.js:31,67,89,97` — `updatePosition`/`setAIScore`/`addOrder`/`addTrade` 对 `null/undefined` 参数做对象展开导致运行时崩溃 | 4 处增加空值守卫 `if (!symbol \|\| !data) return;` | v0.1.0 |
| 2 | 2026-06-14 | 2026-06-14 | 🟡 `examples/Sidenav/index.js:57` — `collapseName` 只取路径第一段，嵌套路由（如 `/futures/browse` → `futures`）与 hyphen 式 key（`futures-browse`）不匹配，路由高亮失效 | 改为 `pathname === '/' ? 'overview' : pathname.slice(1).replace(/\//g, '-')` | v0.1.0 |
| 3 | 2026-06-14 | 2026-06-14 | 🟡 `api/ws.js:248` — `createWS()` 在模块加载时创建 WebSocket 单例（副作用），Jest 环境 `WebSocket is not defined`，且连接在组件挂载前建立 | 改为惰性单例 `getWSManager()`（首次调用时才创建），导出函数而非实例 | v0.1.0 |
| 4 | 2026-06-14 | 2026-06-14 | 🟡 `api/client.js:42` — 错误拦截器用 `new Error(message)` 替换原始错误，丢失 `error.response`（状态码/响应体），调用方无法按状态码分类处理 | 创建 `enhancedError` 保留 `.status` / `.response` / `.originalError` 三个字段 | v0.1.0 |
| 5 | 2026-06-14 | 2026-06-14 | 🟡 `stores/system.js:31` — `alert.timestamp \|\| new Date()` 吞掉空字符串 `''` 或数字 `0`（falsy 陷阱） | 改用 nullish coalescing `??` | v0.1.0 |
| 6 | 2026-06-14 | 2026-06-14 | 🟡 `stores/trading.js:58` — `setAIScore` 全覆盖不合并，第二次调用覆盖第一次的字段 | 改为 ` ...(state.aiScores[symbol] \|\| {}), ...score` 合并模式 | v0.1.0 |
| 7 | 2026-06-14 | 2026-06-14 | 🟡 `stores/trading.js:87,97` — `addOrder`/`addTrade` 无上限增长，活跃交易日内存持续攀升 | 增加 `MAX_ORDERS=500` / `MAX_TRADES=500`，`.slice(0, MAX)` 截断 | v0.1.0 |
| 8 | 2026-06-14 | 2026-06-14 | 🟡 `api/ws.js:196,215` — `connect()` 未清理旧连接（两个 WebSocket 并行存活）+ `close()` 中 `ws.close()` 抛异常导致 `ws` 未置空 | 新增 `closeExistingWS()` 辅助函数，`connect()` 前先清理，`close()` 用 try-finally 确保置空 | v0.1.0 |
| 9 | 2026-06-14 | 2026-06-14 | 🔴 `package.json` — `react-scripts build` 失败：React 17 无 `react-dom/client`（`createRoot`）| 升级 react/react-dom 至 18.2 | v0.1.0 |
| 10 | 2026-06-14 | 2026-06-14 | 🔴 `package.json` — `@tremor/react@3.x` peerDeps 要求 React 18，npm install 失败 | 升级 React → 18.2，补充缺少的 `chroma-js`/`react-icons`/`prop-types` 依赖 | v0.1.0 |
| 11 | 2026-06-14 | 2026-06-14 | 🔴 `assets/theme/base/globals.js` — 引用已删除的 `body-background.png`；`examples/Cards/MasterCard` 引用 `billing-background-card.png`；`examples/Navbars/DashboardNavbar` 引用已删除的 `Breadcrumbs`/`NotificationItem`/demo 图片 | globals.js 改 CSS gradient；MasterCard 改 hardcoded gradient；DashboardNavbar 重写（移除搜索/登录/通知），Configurator 重写（移除社交按钮） | v0.1.0 |
| 12 | 2026-06-14 | 2026-06-14 | 🔴 `jsconfig.json` 缺失 — Vision UI 非相对路径导入（`import X from "App"` 等）无法解析 | 创建 `jsconfig.json`，设 `"baseUrl": "src"` | v0.1.0 |

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 占位初版 |
| v1.1 | 2026-06-14 | 登记「紧急平仓仅在持仓页」的设计风险（accepted 状态） |
| v1.2 | 2026-06-14 | OPT-300 code review 首轮：发现 8 个 P0/P1 问题（对象展开崩溃、路由高亮失效、WS 副作用、错误拦截器、falsy 陷阱、全覆盖、无限增长、竞态条件），12 个全部修复 |
