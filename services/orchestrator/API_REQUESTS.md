# API_REQUESTS.md — 跨服务接口请求登记簿

> ⚠️ **本文档是 orchestrator 与下游服务（主要是 dashboard_web）之间的「接口需求闭环」**。
>
> **dashboard agent**：写代码时如果发现 orchestrator 缺接口，**不要直接写 mock 凑合**，必须在这里登记一条 REQ-XXX。
> **orchestrator agent**：每次接任务前**必须**先读这份文档，把待实现的 REQ-XXX 列入计划。
> **dashboard agent**：每次开新 dashboard 任务前再读一遍，看哪些 REQ-XXX 已实现，把代码里的 TODO 替换成真接口调用。

---

## 阅读顺序

任何 agent 处理本文档前必读：
1. [`/AGENTS.md`](../../AGENTS.md) — 强制工作流 SOP（含「跨服务接口缺口处理」章节）
2. [`/services/orchestrator/README.md`](./README.md) — orchestrator 服务说明（含 REST API 表）
3. [`/services/dashboard_web/README.md`](../dashboard_web/README.md) — dashboard 设计与依赖

---

## 状态机制（三状态闭环）

每条 REQ-XXX 经历三个状态：

| 状态 | 标记 | 谁标记 | 含义 |
|---|---|---|---|
| **待实现** | `[ ]` | dashboard agent 发起 | 需求登记，orchestrator 还没实现 |
| **实现中** | `[~]` | orchestrator agent 认领时 | orchestrator agent 开始写代码 |
| **已实现** | `[x]` | orchestrator agent 完成后 | 接口已上线（含 unit 测试） |
| **已接入** | `[✓]` | dashboard agent 接入后 | dashboard 代码已替换 mock 调真接口 |

**只有进入 `[✓]` 才算真正闭环**。

---

## 优先级

- 🔴 **P0**：dashboard 当前页无法工作，必须立即实现
- 🟡 **P1**：dashboard 当前用 mock 数据，体验受损
- 🟢 **P2**：dashboard 现阶段不需要，但未来需要预留

---

## 当前 REQ 列表

> 用 `## REQ-XXX 标题` 一级章节登记每个请求。

> 模板见本文件最底部。

---

<!-- 这里登记具体 REQ-XXX 任务，按编号顺序 -->

<!-- ==================== REQ 区域开始 ==================== -->

_（暂无登记。dashboard agent 发现接口缺口时往这里加。）_

<!-- ==================== REQ 区域结束 ==================== -->

---

## 工作流

### dashboard agent 的工作流（发现缺口 → 登记 → 临时 mock）

```
1. 写 dashboard 代码时，发现需要某个接口（如 GET /positions/MA2509）
   ↓
2. 检查 orchestrator/README.md 「REST API」表 是否已存在
   ↓
   ├─ 存在 → 直接调用，OK
   └─ 不存在 → 走以下流程
   ↓
3. 检查 API_REQUESTS.md（本文件）是否已登记
   ↓
   ├─ 已登记且状态 [x] → 提醒 orchestrator agent 已实现，但 orchestrator README 未同步（让其同步）
   ├─ 已登记且状态 [ ]/[~] → 在自己代码里加 TODO 注释指向 REQ-XXX，先用 mock 推进
   └─ 未登记 → 在本文件登记一条新 REQ-XXX，然后加 TODO + mock
   ↓
4. 代码注释规范：
   ```
   // TODO(REQ-XXX): 等 orchestrator 实现 GET /positions/{symbol} 后替换 mock
   const position = await fetchMockPosition(symbol);
   ```
   ↓
5. 在 dashboard 任务的「完成报告」中列出**本次任务发起的所有 REQ-XXX**
```

### orchestrator agent 的工作流（认领 → 实现 → 通知）

```
1. 接到 orchestrator 相关任务时，**第一件事**就是读 API_REQUESTS.md
   ↓
2. 把所有 [ ] 待实现的 REQ-XXX 列入「任务报告与计划」
   ↓
3. 等用户确认是否本轮全部实现 / 部分实现
   ↓
4. 认领某条 REQ-XXX → 状态改为 [~]，加 agent 名 + 时间
   ↓
5. 实现接口（写代码 + unit 测试 + 更新 orchestrator/README.md 的 REST API 表）
   ↓
6. 验证（用 curl 测试响应符合 REQ-XXX 中的格式约定）
   ↓
7. 状态改为 [x] 已实现，在条目末尾追加「实现摘要」
   ↓
8. 在「完成报告」中**明确通知 dashboard agent**：
   "REQ-001 / REQ-005 已实现，请在下次 dashboard 任务中接入"
```

### dashboard agent 接入闭环（[x] → [✓]）

```
1. 开新 dashboard 任务时，**第一件事**就是读 API_REQUESTS.md
   ↓
2. 找到所有 [x] 已实现但未 [✓] 接入的 REQ-XXX
   ↓
3. 把对应代码里的 TODO 替换为真实调用
   ↓
4. 跑测试 + /code-review
   ↓
5. 状态改为 [✓] 已接入，记录接入时间
```

---

## 模板（创建新 REQ 时复制）

```markdown
### REQ-XXX 标题 [P0/P1/P2]

**请求方**：dashboard_web (任务 OPT-XXX)
**登记时间**：2026-XX-XX
**状态**：[ ] 待实现 / [~] 实现中 / [x] 已实现 / [✓] 已接入

**场景**：
（说明 dashboard 哪个页面 / 组件需要这个接口，目前用什么 mock 数据）

**需要的接口**：
- 路径：`<METHOD> /api/v1/<path>`
- 查询参数（如有）：____
- 响应格式：
  ```json
  {
    "field1": "...",
    "field2": "..."
  }
  ```
- 边界情况：（404 时返回什么、错误码、空数据等）

**关联**：
- 后端 OPT-XXX（如 OPT-204）
- 前端 OPT-XXX（如 OPT-303）

**验收**：
（dashboard 接入后能完成什么场景）

---

**实现摘要**（orchestrator agent 完成后填写）：
- 实现位置：`services/orchestrator/internal/api/rest/handlers.go`
- gRPC 调用链：orchestrator → trading_agent.GetPositionBySymbol
- unit 测试：`services/orchestrator/tests/unit/positions_test.go`
- 已同步 orchestrator/README.md
```

---

## 与其他文档的关系

| 文档 | 关系 |
|---|---|
| `OPTIMIZATIONS.md` | OPT-XXX 是"任务"，REQ-XXX 是"接口需求"。一个 OPT 可能产生多个 REQ |
| `services/orchestrator/README.md` | REST API 表是**最终事实**，REQ 实现后必须同步到那里 |
| `services/dashboard_web/README.md` | 依赖关系链描述 dashboard 哪个页面依赖哪个 REQ |
| `AGENTS.md` | 第 8.X 章定义这套工作流的强制 SOP |

---

## ⚠️ 禁止行为

| 行为 | 为什么禁止 |
|---|---|
| ❌ dashboard agent 发现缺口直接写死 mock，不登记 | 数据缺口隐藏，后续没人知道 |
| ❌ orchestrator agent 接任务前不读 API_REQUESTS.md | 错过待实现需求 |
| ❌ orchestrator agent 实现接口但忘了改状态为 [x] | 闭环断裂 |
| ❌ orchestrator agent 实现接口但忘了同步 orchestrator/README.md | 接口表不一致 |
| ❌ dashboard agent 接入真接口后忘了改 [✓] | 历史遗留 mock 难追溯 |
| ❌ 跳过本文档直接修改 orchestrator API 表 | 失去缺口的可追溯性 |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-14 | 初版，定义三状态闭环 + 工作流 SOP |
