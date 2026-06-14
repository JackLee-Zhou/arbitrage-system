# OPTIMIZATIONS.md — 全局优化清单

> **本文档面向其他 AI agent**（Claude Code / Cursor / 其他协作 agent）。
> 这里记录**已识别但尚未执行**的代码优化项 + 重构需求。
> 处理完一项 → 在本文档勾掉 + 在底部加变更记录 + 同步更新对应的服务 README。

---

## 阅读顺序

任何 agent 来处理本文档的任一项前，**必须**先读：

1. [`/AGENTS.md`](./AGENTS.md) — ⭐ AI Agent 强制工作流 SOP（第一份读，含任务报告与计划模板）
2. [`/ENGINEERING_GUIDE.md`](./ENGINEERING_GUIDE.md) — 工程规则（含 Rule of Three 等）
3. [`/docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) — 架构方案
4. [`/contracts/README.md`](./contracts/README.md) — 跨服务契约
5. 你要改的具体服务的 README

⚠️ **认领 OPT-XXX 任务也要按 AGENTS.md 第 0 章输出「任务报告与计划」**——不允许跳过。

---

## 状态标记

每项有以下状态之一：

- `[ ]` — 待处理
- `[~]` — 处理中（请在备注里写 agent 名 + 开始时间）
- `[x]` — 已完成（请在底部"变更记录"留痕）
- `[-]` — 已取消（请在备注里说明原因）

---

## 优先级标记

- 🔴 **P0**：阻塞主线开发，必须立即处理
- 🟡 **P1**：影响代码质量但不阻塞，建议尽快处理
- 🟢 **P2**：可选优化，有空再做

---

## 一、跨服务通讯抽取（v1 提出）

### OPT-001 抽重试逻辑到 `pkg/redis/retry.go` 🟡 P1

**背景**：`services/ctp_tick/internal/adapters/redis/writer.go` 中的 `xaddWithRetry` 函数（第 206-222 行）实现了通用的"指数退避重试"逻辑。其他 Go 服务（trading_agent / orchestrator / db_writer）后续都会用到类似模式。

**目标**：把它抽到 `pkg/redis/retry.go`，作为公共辅助函数。

**拆解**：

- [x] **OPT-001-a** 在 `pkg/redis/` 新建 `retry.go`
  - 暴露：`type RetryOpts struct { MaxRetries, InitialBackoff, Multiplier, Timeout }`
  - 暴露：`XAddWithRetry(ctx, c RedisClient, stream string, values map[string]interface{}, opts RetryOpts) error`
  - 暴露：`GetWithRetry(ctx, c RedisClient, key string, opts RetryOpts) (string, error)`
  - 默认值：`MaxRetries=3, InitialBackoff=500ms, Multiplier=2, Timeout=3s`
  - 测试：`pkg/redis/retry_test.go`，用 `MockClient` 模拟失败 / 成功场景
- [x] **OPT-001-b** 改造 `services/ctp_tick/internal/adapters/redis/writer.go`
  - 删除 `xaddWithRetry` 私有函数 + 相关常量
  - 改用 `pkgredis.XAddWithRetry(ctx, w.client, stream, values, retryOpts)`
  - 单元测试不应回归（原有 mock 路径仍然走通）
- [x] **OPT-001-c** 更新 `pkg/redis/README.md`
  - 加 "Retry helpers" 一节，说明用法
- [x] **OPT-001-d** 在 `services/ctp_tick/README.md` 末尾「依赖」一节加上 `pkg/redis.RetryOpts`

**Acceptance Criteria**：

- ctp_tick 的 unit 测试 100% 通过
- 重试 3 次后仍失败的行为与原版一致（错误返回 + 不丢数据）
- 其他服务（trading_agent 等）后续可以**直接用** `pkgredis.XAddWithRetry`

**风险**：

- ⚠️ 改 `writer.go` 注意保留**优雅关闭**逻辑（drain + flushWg）—— 不能因重构破坏
- ⚠️ 别把 ctp_tick 的"满则丢弃"业务策略抽进 pkg —— 那是 ctp_tick 业务

---

### OPT-002 抽服务注册逻辑到 `pkg/registry/` 🟡 P1

**背景**：每个服务都需要做：
1. 启动时 `SET services:<name> <info> EX 30`
2. 后台 goroutine 每 10 秒 `EXPIRE services:<name> 30`
3. 优雅关闭时 `DEL services:<name>`

这是 9 个服务都共享的逻辑。**满足 Rule of Three** —— 应该抽。

**目标**：在 `pkg/registry/` 实现一个 `RegisterAndKeepAlive`。

**拆解**：

- [ ] **OPT-002-a** 新建 `pkg/registry/heartbeat.go`
  - 定义 `type ServiceInfo struct { Name, Version string; StartedAt int64; HealthURL, GRPCAddr string; Capabilities []string }`
  - 暴露 `func RegisterAndKeepAlive(ctx context.Context, client pkgredis.RedisClient, info ServiceInfo) (cancel func(), err error)`
  - 内部逻辑：
    - 立即 `Set("services:"+info.Name, json(info), 30s)` 注册
    - 启动 goroutine 每 10s 续约（用 `Set` 重新写一次或 EXPIRE）
    - cancel() 调用时：停止续约 + DEL key
  - 错误处理：续约失败连续 3 次 → 写日志 WARN，但不停业务（业务不应该因为注册失效而停）
- [ ] **OPT-002-b** 写测试 `pkg/registry/heartbeat_test.go`
  - 用 `pkgredis.NewMockClient()` 验证：
    - 注册成功
    - 续约调用次数 ≈ runtime / 10s
    - cancel() 后 DEL 被调用
- [ ] **OPT-002-c** 在 `pkg/registry/` 写 `README.md`，含 ServiceInfo 字段定义 + 示例
- [ ] **OPT-002-d** 9 个服务的 `cmd/main.go` 都接入（按以下顺序，每改一个跑测试）：
  - ctp_tick
  - trading_agent
  - orchestrator
  - db_writer
  - watchdog
  - intel_collector（Python，不接入 pkg；走 Python 自己的等价实现）
  - llm_scorer（Python，同上）
  - dashboard_web（无主进程，跳过）
  - stock_tick（V2 占位，跳过）

  Go 服务接入示例：

  ```go
  // 不要写代码，这只是给 agent 参考的伪代码
  cancel, err := registry.RegisterAndKeepAlive(ctx, redisClient, registry.ServiceInfo{
      Name:      "ctp_tick",
      Version:   "v0.1.0",
      StartedAt: time.Now().UnixMilli(),
      HealthURL: "http://ctp_tick:8081/healthz",
  })
  defer cancel()
  ```

**Acceptance Criteria**：

- 5 个 Go 服务都使用 `pkg/registry`，无重复代码
- 启动后 `redis-cli KEYS services:*` 能看到全部 5 个
- 任意服务被 kill → 30s 内 key 自动过期消失
- watchdog 通过 `KEYS services:*` 巡检在线服务

**风险**：

- ⚠️ Python 服务（intel_collector / llm_scorer）**不能** import pkg/redis（pkg/ 是 Go 专属）
  → Python 端单独实现一个等价的 `adapters/registry.py`（不属于本任务，但要在两个 Python 服务的 README 加一笔）

---

### OPT-003 业务级写入模式不抽（决策记录） 🟢 P2

**背景**：用户问"跨节点通讯逻辑也需要提取到上层吗"，特指 `ctp_tick` 的 `TickWriter` 中的：
- 异步 channel buffer（防 CTP 回调阻塞）
- 批量聚合（100 条/1s）
- 双写 focus 流
- 优雅关闭（drain + flushWg）

**结论：不抽**。

**理由**：

1. **Rule of Three 未满足**：目前只有 ctp_tick 一处用
2. **db_writer 的批量需求不同**：按 bucket 分类，按时间窗口刷，不是简单的「100 条或 1 秒」
3. **强行抽会越抽越复杂**：泛型 `BatchWriter[T]` 配置项爆炸，调试困难
4. **早期"看起来通用"的代码，第二次用时往往会发现需求不一样**

**何时重新评估**：

- 当 db_writer 完成后，对比它的 `consumer.go` 和 ctp_tick 的 `writer.go`
- 如果发现至少 70% 逻辑相同 → 创建 OPT-XXX 重新评估抽取
- 否则保持现状

**Action Item**：

- [ ] **OPT-003-a** 在 `services/ctp_tick/README.md` 的「AI Agent 准则」加一条：「业务级写入模式（channel buffer + batch + DLQ）暂不抽到 pkg/，等 db_writer 完成后再评估」

---

### OPT-004 gRPC / HTTP 客户端不抽（决策记录） 🟢 P2

**结论：不抽**。

**理由**：

- gRPC：客户端代码 90% 是 protoc 自动生成的（`contracts/generated/go/orchestrator/`），orchestrator 直接 import 即可
- HTTP healthz 巡检：标准库 `net/http` 已经够用，watchdog 只调 8 个 healthz，简单循环

**Action Item**：

- [ ] **OPT-004-a** 在 `services/orchestrator/README.md` 的「依赖」一节明确说明使用 `contracts/generated/go/orchestrator` 自动生成的 gRPC client
- [ ] **OPT-004-b** 在 `services/watchdog/README.md` 的「内部结构」一节明确说明 `healthz_client.go` 用 `net/http` 标准库，不需要额外封装

---

### OPT-005 Python 服务注册（intel_collector + llm_scorer） 🟡 P1

**背景**：OPT-002 抽了 Go 版 `pkg/registry`，但 Python 服务不能 import Go 代码。两个 Python 服务也需要注册到 `services:*` Redis Key 才能被 watchdog 和 orchestrator 发现。

**目标**：在 Python 服务里实现等价的注册 + 续约 + 优雅 DEL 逻辑。

**拆解**：

- [ ] **OPT-005-a** 在 `services/intel_collector/adapters/` 新增 `registry.py`
  - 暴露 `class ServiceRegistry`：方法 `register()` / `keep_alive()` / `unregister()`
  - 用 `redis-py` 客户端
  - 启动后台 `asyncio.Task` 每 10 秒续约
  - 应用退出时（SIGTERM/SIGINT）调用 `unregister()`
- [ ] **OPT-005-b** 在 `services/llm_scorer/adapters/` 新增同样的 `registry.py`（**可选**：直接复制 intel_collector 的版本，OPT-005-a 抽好后参考实现）
- [ ] **OPT-005-c** 两个 Python 服务的 `main.py` 入口接入注册
- [ ] **OPT-005-d** 在两个服务的 README 中登记 `services:<name>` Redis Key

**Acceptance Criteria**：
- 两个 Python 服务启动后 30 秒内 `redis-cli GET services:intel_collector` 返回 JSON
- kill 进程后 30 秒内 key 自动消失（依赖 TTL）
- watchdog 通过 `KEYS services:*` 能看到全部 9 个服务

**风险**：
- ⚠️ 不要 import Go 的 pkg/registry（pkg/ 是 Go 专属）
- ⚠️ Python 优雅退出处理：用 signal handler 调用 `asyncio.run()` 内的 unregister

**关联**：
- OPT-002（Go 版服务注册）
- ENGINEERING_GUIDE.md 规则 7

---

## 二、规则补充（v1 提出）

### OPT-101 ENGINEERING_GUIDE.md 加 Rule of Three 准则 🟡 P1

**背景**：用户提出"跨节点通讯逻辑要不要抽"，本文档已记录决策（OPT-003）。但**长期来看**，需要在工程规则文档里固化这条原则，防止未来 agent 反复犯"过早抽取"的错误。

**拆解**：

- [ ] **OPT-101-a** 在 `ENGINEERING_GUIDE.md` 第 10 章「AI Agent 工作准则」末尾新增一条准则：

  > **准则 9：抽取前问 Rule of Three（不要过早抽取）**
  >
  > 看到"两个服务用法相似"时**不要立刻**抽到 `pkg/`。等到第三处真要复用时再抽。
  >
  > 早期"看起来通用"的代码，第二次用时往往会发现需求不一样，强行抽会越抽越复杂。
  >
  > **可以抽的（满足 Rule of Three 或天然通用）**：
  > - Redis 连接 / Stream / KV 操作（`pkg/redis/`）
  > - 重试包装（指数退避是数学函数）
  > - 服务注册（9 个服务都用同一套）
  >
  > **不要抽的（看似通用其实不通用）**：
  > - 业务级写入模式（batch / channel / DLQ）—— 各服务策略不同
  > - 配置加载 —— 各服务字段不同，Viper 已经够通用
  > - 错误格式化 —— 上下文千差万别

- [ ] **OPT-101-b** 在 `ENGINEERING_GUIDE.md` 文档变更记录加版本号 v1.3 标识本次变更

**Acceptance Criteria**：

- 新增的准则 9 与已有 8 条准则风格一致
- 至少 1 个正例 + 1 个反例

---

## 三、Dashboard 后端配套（v1.2 dashboard_web 设计提出）

> 因 dashboard_web 的「期货合约浏览页」需要展示全市场 ~330 个合约的实时价 + 详情，
> 必须先做以下 3 个后端配套改造。详见 `services/dashboard_web/README.md` 的「期货合约浏览页设计」。

### OPT-201 ctp_tick 写入合约元数据 🟡 P1

**背景**：ctp_tick 启动时通过 TD API 查询 ~330 个合约元数据（代码 / 中文名 / 合约月 / 交易所 / 合约乘数 / 最小变动价位等），但**目前没有持久化**。dashboard 想展示全市场合约列表时无数据来源。

**目标**：ctp_tick 启动后把元数据写到 Redis + InfluxDB 各一份。

**拆解**：

- [ ] **OPT-201-a** ctp_tick 启动流程：TD API 查询完成后，把完整合约列表写入 Redis
  - KEY: `instruments:all` （Hash 或 JSON List）
  - KEY: `instrument:<symbol>` 单个合约元数据
  - 不设 TTL（每次启动覆盖刷新）
- [ ] **OPT-201-b** 同时写一份到 InfluxDB `instruments` bucket（含日期标签）
  - 每月主力切换时记录变化
  - retention 永久
- [ ] **OPT-201-c** 在 `services/ctp_tick/README.md` 的「写入的 Redis」表中登记新增 Key
- [ ] **OPT-201-d** 新增 `contracts/instrument.proto` 定义 `InstrumentMetadata` 消息（如 contracts 中已有则跳过）

**Acceptance Criteria**：
- ctp_tick 启动后 30 秒内 `redis-cli HLEN instruments:all` 返回 ≥ 200
- `redis-cli GET instrument:MA2509` 返回完整 JSON
- InfluxDB query `instruments` bucket 能查到当日数据

**风险**：
- ⚠️ TD API 查询失败时（SimNow 偶发超时）会拿不到合约列表 → 沿用现有降级逻辑（仅订阅 SUBSCRIBE_SYMBOLS）但 dashboard 列表会不全

---

### OPT-202 ctp_tick 维护全市场最新价快照 🟡 P1

**背景**：dashboard 合约列表要展示 330 个合约的"最新价 + 涨跌幅"，但目前只有 SUBSCRIBE_SYMBOLS 中的合约会写到 `tick:<symbol>` Stream。其他合约的 Tick 流过 `write:tick` 后只进 InfluxDB（查询慢）。

**目标**：ctp_tick 在每个 Tick 处理流程中**额外**维护一份最新价快照到 Redis KV，让 dashboard 能秒级查询。

**拆解**：

- [ ] **OPT-202-a** ctp_tick 内部新增 `LastPriceSnapshot` 内存映射 `map[symbol]*PriceSnapshot`
  - 每个 Tick 触发更新（覆盖式）
  - 包含字段：symbol / last_price / open / high / low / pre_close / change_pct / volume / open_interest / updated_at
- [ ] **OPT-202-b** 后台 goroutine 每 1 秒 pipeline 批量写 Redis（节流）
  - KEY: `last_price:<symbol>`（Hash）
  - TTL: 60 秒（断线后自动过期）
- [ ] **OPT-202-c** 提供 `GET /internal/snapshot` HTTP 端点供 orchestrator 全量查询（fallback）
- [ ] **OPT-202-d** 在 `services/ctp_tick/README.md` 的「写入的 Redis」表中登记 `last_price:<symbol>`

**Acceptance Criteria**：
- 接收 1 个 Tick 后，`redis-cli HGETALL last_price:MA2509` 返回最新价信息
- 行情高峰时 Redis 写入频率 ≤ 1 Hz/合约（330 合约 → 总 ≤ 330 ops/s，pipeline 后 ≤ 60 ops/s）
- 60 秒无 Tick 后 KEY 自动过期（避免显示陈旧价）

**风险**：
- ⚠️ Pipeline 批量写入失败时丢数据 → 不致命（下个 Tick 会刷新，且 InfluxDB 有全量）
- ⚠️ 节流频率太高（如 100ms）会增加 Redis 压力，太低（5s）则数据不够实时 → 默认 1s

---

### OPT-203 orchestrator 计算并标记主力合约 🟢 P2

**背景**：dashboard 列表需要"主力合约"标识（每个品种 1 个，约 33 个），用户切「主力」tab 时只显示这些。

**目标**：orchestrator 启动时（或每天定时）计算每个品种的主力合约，写入 Redis。

**拆解**：

- [ ] **OPT-203-a** orchestrator 启动 + 每天 8:30（开盘前）触发计算
  - 从 InfluxDB 查最近 24h 各合约成交量
  - 同品种中**成交量最大**的 = 主力
- [ ] **OPT-203-b** 写入 Redis：
  - KEY: `main_contracts:<exchange>` Set（如 `main_contracts:ZCE`）
  - KEY: `is_main:<symbol>` "1"/"0"（dashboard 列表查询时直接读）
- [ ] **OPT-203-c** 在 dashboard 合约列表的"主力" tab 切换时调用 orchestrator API `GET /api/v1/contracts?type=main`
- [ ] **OPT-203-d** 在 `services/orchestrator/README.md` 中登记新 endpoint + Redis Key

**Acceptance Criteria**：
- orchestrator 启动后 1 分钟内 `redis-cli SMEMBERS main_contracts:ZCE` 返回 ~10 个郑商所主力合约
- 月度主力切换时（如 MA2509 → MA2601），下一个开盘日自动更新

**风险**：
- ⚠️ 启动时 InfluxDB 历史数据不足 → 兜底用预配置的主力清单（`config/default_main_contracts.yaml`）

---

## 四、Dashboard 前端任务（v1.2 dashboard_web 设计提出）

> 每个页面/组件一份 OPT 任务（细粒度）。
> 实施顺序：必须先做完第三章 OPT-201/202/203（后端依赖）才能开 OPT-300 系列前端任务。
> 详见 `services/dashboard_web/README.md`。

### OPT-300 dashboard 脚手架 + Vision UI 模板裁剪 🟡 P1

**背景**：dashboard_web 当前 src/ 为空，需先建立基础工程。

**拆解**：
- [ ] **OPT-300-a** clone Vision UI Dashboard React 到 services/dashboard_web/，复制源码 + 保留 LICENSE_VISION_UI.md
- [ ] **OPT-300-b** 改 package.json name + 删 demo 数据 + 删不需要的 layouts（保留 dashboard 模板）
- [ ] **OPT-300-c** 安装新增依赖：@tremor/react / @tanstack/react-query / zustand / lightweight-charts / tailwindcss（仅作 Tremor 依赖）
- [ ] **OPT-300-d** 配置 Tailwind 不污染 MUI 默认样式（PostCSS scope）
- [ ] **OPT-300-e** 设计令牌（颜色/字体/光晕）固化到 src/theme/tokens.js
- [ ] **OPT-300-f** API 客户端（axios + TanStack Query）+ WebSocket 连接管理
- [ ] **OPT-300-g** Zustand 全局 store：systemStore + tradingStore

**Acceptance Criteria**：
- `npm start` 能跑出 Vision UI 自带的概览页
- WebSocket 客户端能连 orchestrator（用 mock 测试）

---

### OPT-301 顶部导航 + 5 个 tab + 持仓固定栏 🟡 P1

**背景**：dashboard 主框架，含顶部 5 tab + 顶栏右侧持仓固定显示。

**拆解**：
- [ ] **OPT-301-a** 顶部导航组件：5 个 tab（概览 / 交易 / AI / 系统 / 管理）
- [ ] **OPT-301-b** 路由配置（react-router v5）：/, /futures/*, /ai/*, /system/*, /manage/*, /stocks
- [ ] **OPT-301-c** **持仓固定栏**：仅在 /futures/* 路由下顶栏右侧显示（依赖 trading_agent gRPC 持仓）
  - 单合约：`💰 持仓: MA2509 +1 +¥234`
  - 多合约：`💰 总持仓 N 个 总浮动 +¥X`（点击展开）
  - 无持仓：`💰 当前无持仓`
- [ ] **OPT-301-d** 其他 tab（AI/系统/管理）下不显示持仓固定栏

**Acceptance Criteria**：
- 5 个 tab 可正常切换路由
- /futures/* 下顶栏右侧有持仓显示
- /ai/* /system/* /manage/* 下不显示

**依赖**：OPT-204（trading_agent 持仓 gRPC 改为按合约维度）

---

### OPT-302 概览首页（三区汇总） 🟡 P1

**背景**：根据 dashboard README「首页设计」，概览首页是三区汇总（盯盘 + AI + 系统）+ 实时事件流。

**拆解**：
- [ ] **OPT-302-a** 三区卡片布局（左上盯盘区 / 右上 AI 区 / 左下系统区 / 右下事件流）
- [ ] **OPT-302-b** 盯盘区：当前价格 + 当前持仓 + 今日委托/成交次数 → 调 orchestrator overview API
- [ ] **OPT-302-c** AI 区：Live AI 评分 + 老化时间 + 紧急触发事件
- [ ] **OPT-302-d** 系统区：9 个服务健康灯 + 内存/磁盘使用率
- [ ] **OPT-302-e** 实时事件流：WebSocket alerts 频道滚动列表
- [ ] **OPT-302-f** 三个卡片底部"点击进入对应 tab"链接

**Acceptance Criteria**：
- 浏览器访问 / 看到完整三区卡片
- WebSocket 推送事件能实时滚动到事件流卡片

---

### OPT-303 ⭐ 期货合约浏览页（最复杂的一页） 🟡 P1

**背景**：核心交易页面 — 左列表（330 合约 + 搜索 + 持仓置顶）+ 右详情大卡片（分时/日K/盘口五档/持仓）。

**估算**：5-7 个工作日（最复杂）。

**拆解**：
- [ ] **OPT-303-a** 左侧合约列表组件
  - 搜索框（输入代码/中文名/合约月模糊匹配）
  - 持仓中合约置顶（带 ⭐ + 持仓手数 + 浮动盈亏）
  - 列表项：代码 + 中文名 + 最新价 + 涨跌幅
  - 数据源：`GET /api/v1/contracts` + WebSocket prices 频道
- [ ] **OPT-303-b** 详情大卡片组件（点击列表后展示）
  - 顶栏：合约代码 + 中文名 + 最新价 + 涨跌幅（实时跳动）
  - 左上：分时图（TradingView Lightweight Charts，1Hz 推送）
  - 左下：日 K 线（TradingView Charts，可切日/周）
  - 右栏：盘口五档（实时跳动 + 红绿配色）
  - 右下：我的持仓（持仓时显示） + 紧急平仓按钮
- [ ] **OPT-303-c** WebSocket 动态订阅：点击合约 → subscribe `contract:<symbol>` → 离开 → unsubscribe
- [ ] **OPT-303-d** 紧急平仓二次确认弹窗（分两选项：仅平本合约 / 全部平仓+暂停）
- [ ] **OPT-303-e** 实盘环境额外要求输入"确认"二字
- [ ] **OPT-303-f** 全局键盘快捷键 Esc+Esc → 跳转到合约浏览页（依赖 OPT-301）

**Acceptance Criteria**：
- 列表展示 ~330 个合约，最新价实时更新
- 点击 MA2509 → 详情卡片实时显示分时/日K/盘口五档
- 持仓时右下显示"我的持仓"+ 紧急平仓按钮
- 紧急平仓走二次确认（实盘要输入"确认"）

**依赖**：
- OPT-201（合约元数据）
- OPT-202（最新价快照）
- OPT-203（主力合约）
- 后端：orchestrator 6 个新 endpoint
- 后端：trading_agent gRPC 持仓查询

---

### OPT-304 委托 / 成交页 🟢 P2

**拆解**：
- [ ] **OPT-304-a** 当日委托列表（表格 + 筛选 + 分页）→ orchestrator GET /orders
- [ ] **OPT-304-b** 当日成交列表 → orchestrator GET /trades
- [ ] **OPT-304-c** 历史委托/成交查询（按日期范围筛选，P2 简版）

**Acceptance Criteria**：
- 委托/成交数据能实时刷新（WebSocket events 频道）

---

### OPT-305 股票占位页 🟢 P2（极简）

**拆解**：
- [ ] **OPT-305-a** /stocks 路由展示一张占位卡片
  - 标题："📉 股票模块"
  - 内容："状态：规划中（V2 阶段实施）"
  - "计划接入：QMT / easytrader"
  - "预计上线：P3 完成后"
  - 链接：[查看规划文档]（指向 services/stock_tick/README.md）

**Acceptance Criteria**：
- 浏览器访问 /stocks 看到占位卡片

---

### OPT-306 AI 评分 + 历史曲线 🟡 P1

**拆解**：
- [ ] **OPT-306-a** /ai/scores：实时显示 live + shadow 评分 + 老化时间 + 倒计时
- [ ] **OPT-306-b** /ai/scores/history：评分历史曲线（Tremor LineChart）
- [ ] **OPT-306-c** WebSocket scores 频道实时更新

---

### OPT-307 系统监控页（节点状态 + 主机性能 + 审计日志） 🟡 P1

**拆解**：
- [ ] **OPT-307-a** /system/nodes：9 个服务健康度详情 + 关键指标
- [ ] **OPT-307-b** /system/host：CPU/内存/磁盘/网络 实时图表
- [ ] **OPT-307-c** /system/audit：审计日志列表（P1 简版，仅展示最近 100 条）

---

### OPT-308 管理页（策略 / 风控 / 数据源 — V1 只读） 🟢 P2

**拆解**：
- [ ] **OPT-308-a** /manage/strategies：策略列表（**只读**，V1）
- [ ] **OPT-308-b** /manage/risk：风控参数（**只读**，V1）
- [ ] **OPT-308-c** /manage/sources：数据源列表（**只读**，V1）

⚠️ V1 都是**只读**展示。编辑功能在 P3 (OPT-310 见下) 实现。

---

### OPT-309 手机访问兜底页 🟢 P2

**背景**：dashboard 不支持手机端，但万一在手机访问，需要友好提示 + 紧急平仓兜底。

**拆解**：
- [ ] **OPT-309-a** detect 移动端 user agent → 重定向到 /mobile
- [ ] **OPT-309-b** /mobile 页面：
  - 提示"请在桌面浏览器访问"
  - 简化版"紧急平仓"按钮（依赖 OPT-303-d 的二次确认逻辑）

---

### OPT-310 P3 阶段：策略管理 + Monaco YAML 编辑器 🟢 P3 后做

**背景**：P1/P2 都是只读，P3 才实现完整编辑功能。

**拆解**：
- [ ] **OPT-310-a** Monaco Editor 集成（YAML 模式）
- [ ] **OPT-310-b** /manage/strategies 加 CRUD 操作（依赖 orchestrator 的修改型 endpoint + 交易时段守卫）
- [ ] **OPT-310-c** /manage/risk 加风控参数编辑表单
- [ ] **OPT-310-d** /manage/sources 加 RSS 源 CRUD UI

⚠️ 实施前必须确认 orchestrator 的交易时段守卫已就绪。

---

### OPT-311 P3 阶段：舆情看板 + 决策溯源 🟢 P3 后做

**拆解**：
- [ ] **OPT-311-a** /ai/sentiment：舆情指数 + 新闻流 + 情感分布
- [ ] **OPT-311-b** /ai/decisions/[id]：决策溯源页（6 层 ID 反查，机制 6）

---

## 五、跨模块新功能（v1.2 dashboard_web 设计提出）

> 不是单纯的"抽取/重构"，而是**新增功能**。每个任务影响多个模块。

### OPT-204 trading_agent 持仓 gRPC 改为按合约维度 🟡 P1

**背景**：dashboard 合约浏览页需要按合约查询是否持仓 + 持仓详情。当前 trading_agent gRPC 设计是"全部持仓一次性返回"，dashboard 要在前端筛选。改为后端直接支持按 symbol 查询。

**拆解**：
- [ ] **OPT-204-a** `contracts/orchestrator.proto` 新增 `GetPositionBySymbol(symbol)` RPC
- [ ] **OPT-204-b** trading_agent server 实现该方法
- [ ] **OPT-204-c** orchestrator 的 trading_client adapter 加调用方法
- [ ] **OPT-204-d** orchestrator REST `GET /positions/{symbol}` 暴露
- [ ] **OPT-204-e** trading_agent 暴露持仓变化的 gRPC stream（PositionChanged 事件流）
- [ ] **OPT-204-f** orchestrator 转发为 WebSocket positions 频道

**Acceptance Criteria**：
- `curl /api/v1/positions/MA2509` 返回单合约持仓详情
- WebSocket subscribe positions 能实时收到持仓变化

**关联**：OPT-301 / OPT-303 都依赖此

---

### OPT-205 orchestrator 支持 WebSocket 动态订阅 🟡 P1

**背景**：dashboard 合约详情卡片需要"点击合约 → 订阅该合约实时 tick → 离开 → 取消订阅"。当前 orchestrator WebSocket 仅广播全部消息，需扩展支持动态订阅。

**拆解**：
- [ ] **OPT-205-a** orchestrator WebSocket 协议加 `subscribe`/`unsubscribe` 命令
- [ ] **OPT-205-b** 动态频道：`contract:<symbol>`（每个合约一个频道）
- [ ] **OPT-205-c** orchestrator 内部维护 `客户端 → 订阅频道集合` 映射
- [ ] **OPT-205-d** Tick 推送时只发给订阅了该合约频道的客户端
- [ ] **OPT-205-e** 客户端断线时自动清理订阅

**Acceptance Criteria**：
- dashboard 切到 MA2509 详情卡片 → 服务器开始推送该合约 Tick
- dashboard 切到 FU2509 详情卡片 → 自动取消 MA2509 订阅，开始 FU2509
- 100 个客户端同时订阅不同合约 → 互不干扰

**关联**：OPT-303 必须依赖此

---

### OPT-206 trading_agent gRPC 紧急平仓 + 暂停策略 🟡 P1

**背景**：dashboard 紧急平仓按钮要支持两个选项：「仅平本合约」/「全部平仓 + 暂停策略」。当前 trading_agent gRPC 可能只有 EmergencyClose。

**拆解**：
- [ ] **OPT-206-a** `contracts/orchestrator.proto` 中 `EmergencyClose` 加参数 `scope`：
  ```
  enum EmergencyScope {
    SINGLE_CONTRACT = 0;   // 仅平本合约
    ALL_AND_PAUSE   = 1;   // 全部平仓 + 暂停所有策略
  }
  ```
- [ ] **OPT-206-b** trading_agent 实现两种行为
- [ ] **OPT-206-c** orchestrator REST `POST /emergency_close` 接收 scope 参数
- [ ] **OPT-206-d** dashboard 平仓弹窗根据 scope 调用

**Acceptance Criteria**：
- "仅平本合约"：trading_agent 平指定合约持仓，其他不动，策略继续运行
- "全部 + 暂停"：trading_agent 平所有持仓，所有策略 mode 改 paused

**关联**：OPT-303-d 依赖此

---

### OPT-207 watchdog 钉钉告警附带 dashboard 跳转链接 🟢 P2

**背景**：dashboard README 提到"钉钉告警附带快捷链接 → 直跳合约浏览详情卡片"。watchdog 当前钉钉消息是纯文本，需要加 markdown 链接。

**拆解**：
- [ ] **OPT-207-a** watchdog 钉钉消息改为 markdown 格式
- [ ] **OPT-207-b** 持仓告警附带：`[🔗 立即查看 MA2509](http://ali-trade:8000/futures/browse?symbol=MA2509)`
- [ ] **OPT-207-c** 服务挂掉告警附带：`[🔗 节点状态](http://ali-trade:8000/system/nodes)`
- [ ] **OPT-207-d** dashboard URL 通过环境变量 `DASHBOARD_BASE_URL` 配置（避免硬编码）

**Acceptance Criteria**：
- 钉钉群消息可点击链接直跳 dashboard 对应页面

---

## 六、其他已识别的优化（开放区）

> 这一区域用来记录在开发过程中发现但还没立即处理的优化机会。
> 任何 agent 在写代码时发现"这块以后可以优化"，都可以在这里加一条 OPT-XXX。

### 模板

```markdown
### OPT-XXX 标题 [P0/P1/P2]

**背景**：（为什么需要这个优化？）

**目标**：（要做什么？）

**拆解**：
- [ ] **OPT-XXX-a** 子任务 1
- [ ] **OPT-XXX-b** 子任务 2

**Acceptance Criteria**：
- 验收标准 1
- 验收标准 2

**风险**：
- ⚠️ 风险点

**关联**：
- 哪个服务的 README
- ARCHITECTURE.md 的哪一章
```

---

## 七、Agent 处理流程

### 接到一项任务时

1. **认领**：把状态从 `[ ]` 改为 `[~]`，备注里写 agent 名 + 开始时间
   ```markdown
   - [~] **OPT-001-a** 在 pkg/redis/ 新建 retry.go _(claude-code @ 2026-06-13 16:30)_
   ```

2. **读上下文**：
   - 本文档相关章节
   - `ENGINEERING_GUIDE.md` 全文
   - 涉及的服务 README
   - 涉及的源码文件

3. **改代码**：遵循 ENGINEERING_GUIDE.md 全部规则

4. **跑测试**：
   - `pkg/` 下的：`go test ./...`
   - 服务下的：服务内 `go test ./...`
   - 出错就修，不要标完成

5. **更新文档**：
   - 受影响的服务 README
   - `ENGINEERING_GUIDE.md`（如果有规则变化）
   - 本文档：状态从 `[~]` 改为 `[x]`，并在底部「变更记录」加一条

### 完成一项任务后

在本文档底部「变更记录」加：

```markdown
| 日期 | 任务 ID | Agent | 改动摘要 | PR / commit |
|---|---|---|---|---|
| 2026-06-13 | OPT-001-a | claude-code | 新增 pkg/redis/retry.go，定义 XAddWithRetry / GetWithRetry | abc1234 |
```

---

## 八、变更记录

| 日期 | 任务 ID | Agent | 改动摘要 | PR / commit |
|---|---|---|---|---|
| 2026-06-13 | -- | claude-code | 创建 OPTIMIZATIONS.md 初版，登记 OPT-001 ~ OPT-004 + OPT-101 | -- |
| 2026-06-14 | -- | claude-code | 新增第三章「Dashboard 后端配套」：OPT-201 (合约元数据) / OPT-202 (全市场最新价快照) / OPT-203 (主力合约计算)，章节顺序整理 | -- |
| 2026-06-14 | OPT-001-a~d | claude-code | 完成 Redis 重试逻辑抽取：新增 pkg/redis/retry.go + retry_test.go（11 测试全部通过），Mock 加失败注入，ctp_tick 改用 XAddWithRetry | -- |
| 2026-06-14 | -- | claude-code | 新增 OPT-005 (Python 服务注册)；新增第四章「Dashboard 前端任务」12 个 OPT (OPT-300 ~ OPT-311)；新增第五章「跨模块新功能」4 个 OPT (OPT-204 ~ OPT-207)；总 OPT 数 9 → 25 | -- |

---

## 九、不接受的优化（黑名单）

如果 agent 提出以下"优化"，**直接拒绝**：

| 提案 | 拒绝理由 |
|---|---|
| 抽 `pkg/grpc/` 通用客户端 | gRPC 客户端 90% 是 protoc 生成的，没有抽的价值 |
| 抽 `pkg/http/` 通用客户端 | net/http 已经够通用，加层包装是负担 |
| 抽 `pkg/utils/` 工具函数 | 容易变成"垃圾桶包"，每个服务自己写 |
| 抽 `pkg/messaging/` 消息抽象层 | 过度设计，Redis Stream 已经是抽象 |
| 把 domain/ 跨服务共享 | 违反规则 1（每个服务独立） |
| 引入 Consul/Etcd/Nacos | 已在 ARCHITECTURE.md 决策"用 Redis KV 替代" |
| 升级 React 17 → 18 | 已在 dashboard_web README 决策"不主动迁移" |
| 把 dashboard 改回 Go 模板 + HTMX | v2.3 决策已用 Vision UI |
| 把 indicator 拆出 trading_agent | v2.1 决策合并理由充分 |
| 把 dashboard 合并回 orchestrator | v2.2 决策"异构语言无法合并" |

---

## 文档版本

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版，登记 OPT-001（重试抽取）/ OPT-002（注册抽取）/ OPT-003（业务级模式不抽决策）/ OPT-004（gRPC/HTTP 不抽决策）/ OPT-101（加 Rule of Three 准则）|
| v1.1 | 2026-06-14 | 新增第三章「Dashboard 后端配套」：OPT-201 (ctp_tick 写合约元数据) / OPT-202 (ctp_tick 维护全市场最新价快照) / OPT-203 (orchestrator 计算主力合约)；章节顺序整理（原四五六 → 五六七）|
| v1.1 | 2026-06-14 | OPT-001 完成：重试逻辑抽取到 pkg/redis/retry.go，ctp_tick 重构完成 |
| v1.2 | 2026-06-14 | dashboard_web v1.2 设计讨论后整理：(1) 新增 OPT-005 Python 服务注册 (一、跨服务通讯抽取)；(2) 新增第四章「Dashboard 前端任务」12 个 OPT (OPT-300/301/302/303/304/305/306/307/308/309/310/311)，每页一份 OPT 细粒度；(3) 新增第五章「跨模块新功能」4 个 OPT (OPT-204 trading_agent 持仓 gRPC 按合约维度 / OPT-205 orchestrator WebSocket 动态订阅 / OPT-206 紧急平仓两选项 / OPT-207 钉钉链接跳转)；(4) 章节顺序重整为 1-9 章；总 OPT 数 9 → 25 |
