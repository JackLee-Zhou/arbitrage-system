# AGENTS.md — AI Agent 强制工作流 SOP

> **本文档是 AI agent 的"操作手册"。任何在此项目工作的 AI agent（Claude Code / Cursor / 其他）必须遵守。**
>
> 本文档与 `ENGINEERING_GUIDE.md` 的关系：
> - `ENGINEERING_GUIDE.md`：**规则细则**（被动查阅，告诉你"什么可以做"）
> - `AGENTS.md`：**强制 SOP**（主动遵循，告诉你"现在该做什么"）
>
> ⚠️ **如果你只读一份文档，读这份。**
> ⚠️ **如果你跳过了"接任务报告与计划"步骤，用户有权拒绝你的代码。**

---

## 🎯 第一原则（一句话）

> **"先理解，再计划，再实施，最后验收。每一步都向用户报告。"**

---

## 第 0 章：接到任务时**必须**做的事

任何任务（无论大小）开始前，**必须**输出一份「**任务报告与计划**」给用户，然后等用户回复**「确认」/「开始」/「ok」**才能动手写代码。

### 0.1 任务报告与计划模板

**严格按以下模板输出**（缺一项不可）：

```markdown
## 📋 任务报告与计划

### 1. 我理解的任务
（一句话总结用户要我做什么。如果不确定，先问清楚再继续。）

### 2. 我读过的文档
- [ ] AGENTS.md（本文档）
- [ ] ENGINEERING_GUIDE.md 的相关章节：____
- [ ] OPTIMIZATIONS.md 是否有相关 OPT-XXX：____
- [ ] docs/ARCHITECTURE.md 的相关章节：____
- [ ] 涉及服务的 services/<name>/README.md：____
- [ ] contracts/ 是否需要改：____
- [ ] services/orchestrator/API_REQUESTS.md（dashboard / orchestrator 任务必读）：____

### 3. 改动范围
- 新增文件：____
- 修改文件：____
- 涉及服务（可能多个）：____
- 涉及契约（contracts/*.proto）：____ / 不涉及

### 4. 我的实施计划（按顺序）
1. ____
2. ____
3. ____

### 5. 我会触碰的硬规则
（对照 ENGINEERING_GUIDE.md 的 7 条硬规则 + 10 条 AI Agent 准则，逐条回答会不会触碰。）
- 规则 1（服务独立部署）：____
- 规则 2（contracts 通信）：____
- 规则 3（domain 零依赖）：____
- 规则 6（禁止魔法数字）：____
- 规则 7（pkg/redis 统一）：____
- 准则 9（Rule of Three）：____

### 6. 风险点 / 不确定项
- ⚠️ ____
- ⚠️ ____

### 7. Definition of Done（完成定义）
完成后我会做这些事：
- [ ] 改动的服务跑 unit 测试
- [ ] 跑 make check / golangci-lint / ruff
- [ ] ⭐ **跑 `/code-review`**（首轮 + 修复后二次）
- [ ] ⭐ 把发现的问题登记到 `services/<name>/QUESTION.md` + `docs/QUESTION_DEAL.md`
- [ ] ⭐ 修复 🔴 P0 + 🟡 P1，在 QUESTION.md「已修复」表标注修复方式
- [ ] 更新涉及服务的 README
- [ ] 如改了契约：跑 buf lint / make breaking
- [ ] 如发现新优化机会：写到 OPTIMIZATIONS.md
- [ ] 报告给用户：哪些跑通、哪些没跑、评审情况、为什么

### 8. 等待用户确认
请确认是否开始。如有疑问请提出。
```

### 0.2 强制要求

1. **不允许跳过这一步**。
2. **不允许在用户没确认前写一行代码**。
3. **不允许"我理解错了任务"作为事后理由**——如果不确定，第 1 项就要问清楚。

### 0.3 例外（仅以下情况可以跳过 0.1）

- 用户明确说「不用计划，直接写」
- 改动是 1 行的 typo 修复
- 用户已经在前一轮对话里指定了详细做法

---

## 第 1 章：阅读顺序（任务认领前必读）

```
1. AGENTS.md（本文档）              ⭐ 工作流 SOP
2. ENGINEERING_GUIDE.md             — 规则细则
3. OPTIMIZATIONS.md                 — 看有没有现成 OPT-XXX 任务
4. docs/ARCHITECTURE.md             — 架构（按需查相关章节）
5. contracts/README.md              — 契约规则（如改契约）
6. services/<你要改的模块>/README.md  — 该服务详情
```

⚠️ **不要"全文背诵"** — 用 grep / 跳读关键章节即可。

---

## 第 2 章：核心规则 Cheat Sheet

> 以下是从 `ENGINEERING_GUIDE.md` 浓缩的"红线"，违反任何一条 = PR 拒绝。

### 7 条硬规则（速查表）

| # | 规则 | 一句话 | 违反信号 |
|---|---|---|---|
| 1 | **服务独立** | 每个服务独立 Docker 镜像 + 独立部署 | 在多个服务里 import 同一份 domain 代码 |
| 2 | **契约通信** | 跨服务只走 `contracts/*.proto` | 手写 cross-service struct |
| 3 | **domain 零依赖** | 业务逻辑不 import Redis/HTTP/IO | domain/ 出现 `import "redis"` |
| 4 | **必有 healthz/metrics** | 每个服务暴露 /healthz + /metrics | 新服务没有这两个端点 |
| 5 | **unit 测试脱网** | `go test ./...` 不需要 Redis/网络 | 测试要起 docker-compose 才能跑 |
| 6 | **禁止魔法数字** | 除 0/1/-1 外都要具名常量 | `time.Sleep(30 * time.Second)` 而非 `CTPSessionStartDelay` |
| 7 | **Redis 走 pkg/redis** | 禁止 `redis.NewClient()` | 服务里 `import "github.com/redis/go-redis/v9"` |

### 10 条 AI Agent 准则（速查表）

| # | 准则 | 一句话 |
|---|---|---|
| 1 | 先读再写 | 写代码前先读相关 README |
| 2 | 跨服务改动 = 改契约 | 两个服务都要改？先问"是不是契约要更新" |
| 3 | domain 不 import IO | 看到这种 import 立刻挪到 adapters/ |
| 4 | 不要过度设计 | 没在 ARCHITECTURE.md 里的依赖不要加 |
| 5 | 不改硬决策 | alpha=0.2、9 个服务、全云部署等不可偷改 |
| 6 | 测试优先 | domain 改动必须有 unit 测试 |
| 7 | 保留可追溯性 | 6 层 ID 血缘不可破坏（机制 6） |
| 8 | 报告事实 | 测试失败就说失败，不要回避 |
| 9 | Rule of Three | 不要过早抽取，第 3 处用时再抽 |
| 10 | 写 OPTIMIZATIONS | 发现新优化机会写到 OPTIMIZATIONS.md，不要现在改 |

---

## 第 3 章：写代码前的自检清单（10 问）

动手前对照下面 10 个问题，逐条问自己：

```
1. [ ] 我读过本服务的 README 吗？
2. [ ] 我要改的代码在 domain/ 还是 adapters/？
       └─ 在 domain/：确认不会 import 任何 IO 库
       └─ 在 adapters/：确认通过 interface 接收依赖
3. [ ] 我要新增 Redis Key / Stream 吗？
       └─ 是 → 在服务 README + ENGINEERING_GUIDE.md 第 4 章命名规范登记
4. [ ] 我要新增环境变量吗？
       └─ 是 → 同步更新 .env.example + 服务 README
5. [ ] 我要改契约（contracts/*.proto）吗？
       └─ 是 → 跑 buf lint / make breaking，更新两端服务
6. [ ] 我要新增魔法数字吗？
       └─ 是 → 必须定义具名常量
7. [ ] 我要新增依赖（go.mod / pyproject.toml）吗？
       └─ 是 → 在计划报告里标明，让用户确认
8. [ ] 我的代码会跨服务共享吗？
       └─ 是 → 数一下用过几次，不到 3 处不要抽到 pkg/
9. [ ] 我会写 unit 测试吗？
       └─ domain/ 改动：必须写
       └─ adapters/ 改动：尽量写（用 mock）
10. [ ] 我清楚这个改动的 Definition of Done 吗？
       └─ 不清楚 → 回到第 0 章重新写计划
```

---

## 第 4 章：代码评审与问题闭环（强制环节）

> **代码写完不等于完事**。在告诉用户「完成」之前，**必须**走完一轮"自我评审 + 问题登记 + 修复 + 二次验证"的闭环。

### 4.1 强制流程（一图说清）

```
代码写完 + Unit 测试通过
        ↓
┌──────────────────────────────────┐
│ Step 1: 调用 code review skill    │
│   /code-review high               │  ← 触发审查
└──────────────────────────────────┘
        ↓
   评审结果（含若干 issue）
        ↓
┌──────────────────────────────────┐
│ Step 2: 登记问题                  │
│   — 写入 services/<name>/QUESTION.md │  ← 模块级
│   — 同步 docs/QUESTION_DEAL.md       │  ← 全局汇总
└──────────────────────────────────┘
        ↓
┌──────────────────────────────────┐
│ Step 3: 修复问题                  │
│   按严重度逐项修复（🔴 → 🟡 → 🟢） │
└──────────────────────────────────┘
        ↓
┌──────────────────────────────────┐
│ Step 4: 在 QUESTION.md 标注修复    │
│   写入"已修复问题"表格             │
│   含：修复时间 / 修复方式 / 版本号  │
└──────────────────────────────────┘
        ↓
┌──────────────────────────────────┐
│ Step 5: 二次评审                  │
│   再跑一次 /code-review            │
│   确认问题已闭环（无新增 P0/P1）    │
└──────────────────────────────────┘
        ↓
   走第 5 章 Definition of Done
```

⚠️ **任何一步跳过 = 任务未完成**。

### 4.2 Step 1：调用 code review skill

#### 触发命令

```
/code-review high
```

或在对话中说：

```
请用 code review skill 审查我刚才的改动。
```

#### review 选项的选择

| 选项 | 何时用 |
|---|---|
| `low` / `medium` | 改动 < 50 行，少量修复 |
| **`high`**（推荐） | 改动 ≥ 50 行，新增功能 |
| `max` | 关键服务（trading_agent / db_writer）的核心逻辑改动 |

⚠️ **不要为了"快"选 low**——重要服务必须 high 起步。

#### 审查范围

review skill 会检查：
- 正确性 bug（数据竞争、空指针、边界条件）
- 与 ARCHITECTURE.md / ENGINEERING_GUIDE.md 不一致的地方
- domain/ 是否 import 了 IO（违反规则 3）
- 魔法数字（违反规则 6）
- 错误处理是否完整
- 单元测试覆盖率
- 资源泄漏（goroutine / channel / file handle）

### 4.3 Step 2：登记问题到 QUESTION.md

#### 模块级：`services/<name>/QUESTION.md`

**必须写入「当前待解决问题」表**：

```markdown
| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 25 | 2026-06-14 | 描述具体问题（含文件:行号） | 🔴 | open |
| 26 | 2026-06-14 | ... | 🟡 | open |
| 27 | 2026-06-14 | ... | 🟢 | open |
```

#### 严重度判定

| 标记 | 含义 | 例子 |
|---|---|---|
| 🔴 P0 | 必须立即修，关系正确性 / 资金安全 / 数据一致性 | 数据竞争、风控被绕过、CTP 字段错位、内存泄漏 |
| 🟡 P1 | 必须修，关系稳定性 / 可维护性 | 错误未检查、超时缺失、重复代码 |
| 🟢 P2 | 改进性，可推迟 | 命名不规范、多余 import、注释质量 |

#### 全局级：`docs/QUESTION_DEAL.md`

**同步更新**「服务 → QUESTION.md → 待解决 / 已修复」汇总表：

```markdown
| 服务 | QUESTION.md | 待解决 | 已修复 | 状态 |
|---|---|---|---|---|
| ctp_tick | [QUESTION.md](../services/ctp_tick/QUESTION.md) | 7 | 24 | 开发中 |
                                                     ↑
                                              加上本次新增 issue 数
```

### 4.4 Step 3：逐项修复

修复顺序：**🔴 → 🟡 → 🟢**

修复原则：
- 🔴 不修就不能交付
- 🟡 必须修，除非有充分理由（用户同意推迟）
- 🟢 时间充裕则修，否则在 QUESTION.md 中保留 open 状态

⚠️ **绝不**：为修而修破坏可读性 / 引入新 bug。

### 4.5 Step 4：修复后在 QUESTION.md 闭环登记

把**已修复**的问题从「当前待解决问题」表移动到「已修复问题」表，**必须**包含：

```markdown
| # | 发现时间 | 修复时间 | 描述 | 修复方式 | 版本 |
|---|---|---|---|---|---|
| 25 | 2026-06-14 | 2026-06-14 | 🔴 描述 | **具体的修复方法**（不是泛泛说"已修复"） | v0.2.4 |
```

#### 修复方式怎么写（重点）

| ❌ 不合格 | ✅ 合格 |
|---|---|
| "已修复" | "改用 `time.ParseInLocation` + `Asia/Shanghai`" |
| "添加了检查" | "writer.go:178 加 `if err != nil` 早返回 + 错误日志" |
| "重构" | "把 `xaddWithRetry` 抽到 `pkg/redis/retry.go`，原函数改调辅助" |

⚠️ **修复方式必须能让 6 个月后的别人（包括你自己）一眼看懂当时怎么改的**。

### 4.6 Step 5：二次评审

修复完后**必须**再跑一次：

```
/code-review high
```

#### 二次评审通过标准

- ✅ 没有新增 🔴 P0 问题
- ✅ 已修复的问题 review 不再报告
- ⚠️ 如发现新的 🟡 P1：判断是否当次修复，或登记到 QUESTION.md 留下次

#### 如果二次评审仍有 🔴 P0

→ 回到 Step 3 继续修，**不允许直接交付**

### 4.7 例外（可以跳过本章的情况）

- ❌ **绝不**：用户没明确说"跳过 review" 不能擅自跳
- ✅ **可以跳**：1 行 typo 修复 / 仅改文档 / 仅改 README
- ✅ **可以跳**：用户明确说「这次不用 review，直接交」（但要在完成报告里标记）

### 4.8 给用户的报告格式

完成报告中**必须**包含本章相关信息：

```markdown
## 🔍 代码评审情况
- [ ] /code-review 已跑（first round）：发现 N 个问题
  - 🔴 P0：N 个
  - 🟡 P1：N 个
  - 🟢 P2：N 个
- [ ] QUESTION.md 已登记：services/<name>/QUESTION.md 第 N-M 项
- [ ] docs/QUESTION_DEAL.md 已同步：待解决 X → Y，已修复 A → B
- [ ] 已修复：N 个（具体见 QUESTION.md 已修复表）
- [ ] 推迟修复（保留 open）：N 个，理由：____
- [ ] /code-review 二次评审：✅ 无新增 P0 / ❌ 仍有 P0（继续修）
```

---
---

## 第 5 章：Definition of Done（完成定义）

代码改完不等于任务完成。**必须**走完以下 14 个出口才能告诉用户「完成」：

### 5.1 代码层面（5 个）

- [ ] 改动的服务能编译通过（`go build ./...` / Python 能 import）
- [ ] 改动的服务 unit 测试 100% 通过（`go test ./...` / `pytest tests/unit/`）
- [ ] lint / format 通过（`golangci-lint run` / `ruff check` / `black --check`）
- [ ] 没有引入循环依赖
- [ ] 没有硬编码密钥 / 测试数据

### 5.2 契约层面（2 个，如适用）

- [ ] 如改了 `contracts/*.proto`：`buf lint` + `make breaking` 通过
- [ ] 如改了契约：两端服务（生产者 + 消费者）都更新了

### 5.3 ⭐ 评审层面（3 个，第 4 章详述）

- [ ] `/code-review` 已跑（首轮 + 修复后二次）
- [ ] 发现的问题已登记到 `services/<name>/QUESTION.md` + 同步 `docs/QUESTION_DEAL.md`
- [ ] 🔴 P0 + 🟡 P1 全部修复（🟢 P2 可保留 open，但需用户同意）

### 5.4 文档层面（2 个）

- [ ] 服务 README 同步（如新增 Redis Key / 端点 / 环境变量）
- [ ] `.env.example` / `ENGINEERING_GUIDE.md` / `OPTIMIZATIONS.md` 同步（按需）

### 5.5 报告层面（2 个）

- [ ] 给用户输出「**完成报告**」（见 5.6 模板）
- [ ] 报告中**实事求是**：哪些跑通了、哪些没跑、跳过的为什么跳过

### 5.6 完成报告模板

```markdown
## ✅ 任务完成报告

### 1. 任务回顾
（用户原始要求一句话）

### 2. 实际改动
- 新增文件：____
- 修改文件：____
- 删除文件：____

### 3. 验证结果
- [ ] 编译：✅ / ❌（报错信息）
- [ ] Unit 测试：✅ N/N / ❌（哪个挂了）
- [ ] Lint：✅ / ⚠️（警告但未修复，说明原因）/ ❌
- [ ] 契约（如适用）：✅ buf lint / make breaking 通过
- [ ] 集成测试（如适用）：✅ / ⏭️ 跳过（说明原因）

### 4. 文档同步
- [ ] services/<name>/README.md：____
- [ ] OPTIMIZATIONS.md：____
- [ ] ENGINEERING_GUIDE.md：____

### 5. ⚠️ 遗留问题
（如有未解决的事，必须列出，不能隐瞒）
- ____

### 6. 偏离计划之处
（如改动范围与第 0 章计划不一致，必须说明）
- ____
```

---

## 第 6 章：失败处理决策树

> 写代码过程中遇到问题，按以下决策树处理。**绝不要"硬撑"或"跳过"**。

### 6.1 编译失败

```
编译报错？
  ├─ 是缺少依赖（import 错） → 修复 import，继续
  ├─ 是类型错误（field 不匹配） → 检查是否要改契约，参考第 7 章异议机制
  └─ 是语法错误 → 修复，继续
```

**绝不**：注释掉报错的代码"先跑起来"。

### 6.2 Unit 测试失败

```
测试失败？
  ├─ 是我的代码错 → 修代码
  ├─ 是测试本身写错（边界值算错等） → 修测试
  ├─ 是测试依赖外部服务（违反规则 5） → 改用 mock
  └─ 不知道为什么失败 → ⚠️ 上报用户，附：
                          - 失败的测试名
                          - 报错信息
                          - 我尝试过什么
                          - 我的猜测
```

**绝不**：删除失败的测试 / 改 assertion 让它通过 / 加 `t.Skip()` 跳过。

### 6.3 Lint 警告

```
golangci-lint / ruff 警告？
  ├─ 是 critical 级别（错误） → 必须修
  ├─ 是 warning（如未使用变量） → 修
  ├─ 是 info（如行太长） → 可酌情修，不要为修而修破坏可读性
  └─ 是误报（误判） → 加 //nolint 注释 + 在完成报告里说明
```

### 6.4 集成测试 / 冒烟测试失败

```
集成失败？
  ├─ 是 Docker 起不来 → 检查 .env / docker-compose 配置
  ├─ 是依赖服务挂了 → 重启或检查依赖
  ├─ 是真实业务问题 → 上报用户（这通常是 bug）
  └─ 是网络问题 / SimNow 节点维护 → ⏭️ 跳过 + 报告时说明
```

### 6.5 契约 breaking change

```
buf breaking 报错？
  ├─ 是真的破坏向后兼容 → ⚠️ 上报用户，**绝不强推**
  └─ 是误报（如新增字段被误判） → 检查 buf.yaml 配置
```

**绝不**：跳过 buf breaking 检查 / 改字段编号绕过检测。

### 6.6 不确定时怎么办

**模板**：

```markdown
## ⚠️ 我遇到了问题

### 问题
（一句话描述）

### 上下文
（我在做什么任务、改到哪一步、报错信息）

### 我尝试过的
1. ____
2. ____

### 我的猜测
（最可能的原因）

### 我想做的
A. ____ （选项 A）
B. ____ （选项 B）

请告诉我：
1. 我的猜测对吗？
2. A 还是 B，或者你有更好建议？
```

---

## 第 7 章：异议机制（规则不对怎么办）

> 你（agent）有时候会觉得"这条规则不对"或"这次该破例"。**正确的做法不是偷偷绕过**。

### 6.1 三种情况对应三种做法

| 情况 | 做法 |
|---|---|
| **规则字面如此，但场景特殊** | 在「任务报告」第 6 项「风险点」里说明，等用户决定 |
| **规则可能过时了** | 写到 `OPTIMIZATIONS.md` 第三区「开放区」，提案修改规则 |
| **规则有内部矛盾**（不同文档冲突） | 上报用户，要求统一文档 |

### 6.2 异议模板

```markdown
## 🤔 我对规则有异议

### 涉及规则
（引用 ENGINEERING_GUIDE.md 第 X 章规则 Y / OPTIMIZATIONS.md 黑名单第 Z 项）

### 我看到的场景
____

### 我认为应该
____

### 理由
1. ____
2. ____

### 我的请求
A. 你同意我破例 + 我在完成报告里说明
B. 修改规则 + 我顺手改 ENGINEERING_GUIDE.md
C. 你有更好建议
```

⚠️ **绝不**：自作主张破例 + 不告诉用户。

---

## 第 8 章：常见任务类型 SOP

> 每个任务流程都包含 ⭐ **code review 闭环**（详见第 4 章）。
> 「写代码 → 跑测试 → /code-review → 登记 QUESTION → 修复 → 二次评审 → 完成报告」是**所有任务的通用尾部**。

### 8.1 「认领 OPTIMIZATIONS.md 的 OPT-XXX」

```
1. 读 OPTIMIZATIONS.md 找到 OPT-XXX
2. 把状态从 [ ] 改为 [~]，加 agent 名 + 时间
3. 输出第 0 章「任务报告与计划」（参考 OPT-XXX 的拆解）
4. 等用户确认
5. 写代码
6. 跑测试（unit + lint）
7. ⭐ /code-review high → 登记 QUESTION.md + docs/QUESTION_DEAL.md（第 4 章）
8. ⭐ 按 🔴 → 🟡 → 🟢 顺序修复 → 在 QUESTION.md 标注修复方式
9. ⭐ 二次 /code-review → 确认无新增 P0
10. 输出第 5 章 5.6「完成报告」（含评审报告）
11. 把 OPT-XXX 状态从 [~] 改为 [x]
12. 在 OPTIMIZATIONS.md 底部「变更记录」加一行
```

### 8.2 「修复 bug」

```
1. 复现 bug（必须能在测试里复现，否则修了也不知道修没修）
2. 写一个失败的 unit 测试（先红）
3. 输出第 0 章「任务报告与计划」
4. 等用户确认
5. 改代码（让测试通过，绿）
6. 跑全部 unit 测试（确认没回归）
7. ⭐ /code-review medium → 登记 QUESTION + 修复 + 二次评审（第 4 章）
8. 输出第 5 章 5.6「完成报告」（含评审报告）
```

### 8.3 「新增功能」

```
1. 读相关服务 README
2. 看是否要改契约
3. 输出第 0 章「任务报告与计划」（必须含影响范围、Definition of Done）
4. 等用户确认
5. 改契约（如有）→ make gen
6. 改代码（domain → adapters → api）
7. 写 unit 测试（domain 必须）
8. 同步 README
9. ⭐ /code-review high → 登记 QUESTION + 修复 + 二次评审（第 4 章）
10. 输出第 5 章 5.6「完成报告」（含评审报告）
```

### 8.4 「重构 / 抽取」

```
0. 先问 Rule of Three：是不是已经在 3 处用过了？
   └─ 没满足 → 不要抽，回去做别的
1. 读 OPTIMIZATIONS.md 看是否已登记
   └─ 没登记 → 先登记 OPT-XXX，让用户决定优先级
   └─ 已登记 → 认领，按 8.1 流程
2. 严格遵守"原行为不变"原则：
   └─ 重构前先把现有代码的 unit 测试跑过
   └─ 重构后这些测试必须仍然通过
3. 输出第 0 章「任务报告与计划」
4. 等用户确认
5. 改代码 + 跑测试
6. ⭐ /code-review high → 登记 QUESTION + 修复 + 二次评审（第 4 章）
   ⚠️ 重构特别注意：检查是否引入新 bug（这是重构最大风险）
7. 输出第 5 章 5.6「完成报告」（含评审报告）
```

### 8.5 ⭐「dashboard 发现缺接口」（跨服务接口缺口处理）

> 适用场景：写 dashboard 代码时，发现需要某个 orchestrator REST 接口但**当前不存在**。

```
1. 检查 services/orchestrator/README.md 「REST API」表是否已存在
   ├─ 已存在 → 直接调用，跳过下面流程
   └─ 不存在 → 继续

2. 检查 services/orchestrator/API_REQUESTS.md 是否已登记
   ├─ 已登记且状态 [x] 已实现 → 提醒用户："orchestrator README 未同步该接口"
   ├─ 已登记且状态 [ ]/[~] → 在自己代码加 TODO 注释指向 REQ-XXX，先用 mock 推进
   └─ 未登记 → 在 API_REQUESTS.md 登记一条新 REQ-XXX

3. 登记新 REQ-XXX 时必须包含：
   - 请求方（哪个 OPT-XXX 触发）
   - 场景描述（哪个页面 / 组件需要）
   - 接口规格（路径 / 参数 / 响应 JSON / 边界情况）
   - 优先级（🔴 P0 / 🟡 P1 / 🟢 P2）

4. dashboard 代码中加 TODO 注释规范：
      // TODO(REQ-001): 等 orchestrator 实现 GET /positions/{symbol} 后替换 mock
      const position = await fetchMockPosition(symbol);

5. 完成报告中**必须**列出本次任务发起的所有 REQ-XXX：
   "## 本次任务发起的接口需求
    - REQ-005: GET /positions/{symbol}（已加 mock 推进）
    - REQ-006: WebSocket subscribe contract:<symbol>（已加 mock 推进）"
```

### 8.6 ⭐「orchestrator 实现接口」（响应缺口）

> 适用场景：接到 orchestrator 任务时，**第一件事**就是检查 API_REQUESTS.md。

```
1. 读 services/orchestrator/API_REQUESTS.md
   找出所有 [ ] 待实现的 REQ-XXX

2. 在「任务报告与计划」第 4 项里列出本次要实现哪些 REQ-XXX

3. 等用户确认（用户可能：本轮全做 / 部分做 / 推迟）

4. 认领时把 REQ-XXX 状态从 [ ] 改为 [~]，加 agent 名 + 时间

5. 实现：
   - 写代码（adapters → api/rest）
   - 写 unit 测试
   - 用 curl 验证响应符合 REQ 中的格式约定
   - 同步更新 services/orchestrator/README.md 的 REST API 表

6. ⭐ /code-review high → 登记 QUESTION + 修复 + 二次评审（第 4 章）

7. 实现完成后：
   - REQ-XXX 状态改为 [x]
   - 在 REQ-XXX 末尾追加「实现摘要」（实现位置 / gRPC 调用链 / 测试 / 文档）
   - 在 dashboard agent 接入前**不要**改 [✓]

8. 输出第 5 章 5.6「完成报告」时**明确**通知：
   "## 已实现的接口需求（待 dashboard 接入）
    - REQ-001: GET /positions/{symbol} ✅
    - REQ-005: WebSocket contract:* 频道 ✅
    请在下次 dashboard 任务中按 AGENTS.md 第 8.7 章接入"
```

### 8.7 ⭐「dashboard 接入已实现接口」（闭环）

> 适用场景：开新 dashboard 任务时（任何任务），**第一件事**就是检查是否有待接入的接口。

```
1. 读 services/orchestrator/API_REQUESTS.md
   找出所有 [x] 已实现但未 [✓] 接入的 REQ-XXX

2. 在「任务报告与计划」中列出：
   - 本次任务的主要目标（如 OPT-303）
   - 顺手要接入的 REQ-XXX 清单

3. 写代码时把 TODO 替换为真实调用：
      // 之前：
      // TODO(REQ-001): 等 orchestrator 实现 GET /positions/{symbol} 后替换 mock
      const position = await fetchMockPosition(symbol);
      
      // 之后：
      const position = await api.get(`/api/v1/positions/${symbol}`);

4. 跑测试 + /code-review

5. 完成报告中标注：
   "## 已接入的接口需求
    - REQ-001 [✓] GET /positions/{symbol} 已替换 mock
    - REQ-005 [✓] WebSocket contract:* 频道 已替换 mock"

6. 把 API_REQUESTS.md 中对应 REQ-XXX 的状态改为 [✓]，记录接入时间
```

### 8.8 「写文档」

```
1. 看类似文档的结构（同级文档保持风格统一）
2. 输出第 0 章「任务报告与计划」（说明文档大纲）
3. 等用户确认
4. 写文档
5. （文档不需要 /code-review，但需要自查：链接是否正确、格式是否统一、与其他文档不矛盾）
6. 输出第 5 章 5.6「完成报告」
```

⚠️ 注意：**8.1-8.4 + 8.6 + 8.7 都是代码任务，必须走 code review 闭环。8.5 是工作流登记任务（不写代码）。8.8 文档任务可豁免**。

---

## 第 9 章：禁止的"小聪明"

以下是常见的"agent 自作主张"行为，**全部禁止**：

| 行为 | 为什么禁止 |
|---|---|
| ❌ 没有计划报告就写代码 | 跳过用户审核 |
| ❌ 测试失败就 `t.Skip()` | 隐瞒问题 |
| ❌ 加 `//nolint:all` 屏蔽全部 lint | 掩盖坏味道 |
| ❌ 在 domain/ 里悄悄 import IO | 违反规则 3 |
| ❌ 改 ARCHITECTURE.md 的硬决策 | 违反准则 5 |
| ❌ 看到"两个服务相似"立即抽 pkg | 违反 Rule of Three（准则 9） |
| ❌ 加新依赖不在计划里说 | 跳过用户审核 |
| ❌ "顺手"改无关代码 | 增加 review 难度 |
| ❌ "我觉得 alpha 应该 0.3" 改值 | 违反准则 5（不改硬决策） |
| ❌ 完成报告说"全部通过"实际有跳过 | 违反准则 8（报告事实） |
| ❌ 创建未在 ARCHITECTURE.md 里的新服务 | 违反准则 4（不要过度设计） |
| ❌ 写代码前不读 services/<name>/README.md | 违反准则 1 |
| ❌ 跳过 `/code-review` 直接交付（除文档任务外） | 违反第 4 章强制流程 |
| ❌ /code-review 报问题但不登记 QUESTION.md | 隐瞒问题 |
| ❌ QUESTION.md 修复方式写"已修复"敷衍 | 违反"修复方式必须能让别人一眼看懂" |
| ❌ 二次 review 还有 P0 但仍交付 | 违反第 4 章 4.6 |
| ❌ 把已修复问题留在 QUESTION.md「待解决」表 | 状态不一致 |
| ❌ dashboard 发现缺接口直接写死 mock 不登记 API_REQUESTS.md | 数据缺口隐藏，后续没人知道 |
| ❌ orchestrator 接任务前不读 API_REQUESTS.md | 错过 dashboard agent 提的需求 |
| ❌ orchestrator 实现接口后忘了改 REQ-XXX 状态为 [x] | 闭环断裂 |
| ❌ dashboard 接入真接口后忘了改 REQ-XXX 状态为 [✓] | 历史遗留 mock 难追溯 |
| ❌ 跳过 API_REQUESTS.md 直接修改 orchestrator REST API 表 | 失去缺口的可追溯性 |

---

## 第 10 章：自我提问检查（每次任务结束前问自己）

任务完成报告输出前，**必须**对自己问以下问题。任何一项答案是「不」或「不确定」，**回去补完再说完成**：

```
[ ] 我有没有跳过第 0 章的任务报告与计划？
[ ] 我读过涉及服务的 README 吗？
[ ] 我有没有触碰 7 条硬规则中的任何一条？
    （如有 → 在完成报告说明 + 提供异议）
[ ] 我有没有触碰 10 条 AI 准则中的任何一条？
[ ] 我的改动有 unit 测试吗？测试通过了吗？
[ ] 我有没有跑 lint？警告处理了吗？
[ ] 我改了契约吗？两端都更新了吗？
[ ] 我新增了 Redis Key / 环境变量 / 端口吗？文档同步了吗？
[ ] 我加了新依赖吗？在计划里说过吗？
[ ] ⭐ 我跑过 /code-review 了吗？首轮 + 修复后二次？
[ ] ⭐ 发现的问题登记到 QUESTION.md + docs/QUESTION_DEAL.md 了吗？
[ ] ⭐ 修复方式我写得够具体吗？6 个月后我自己能看懂吗？
[ ] ⭐ 二次 review 还有 P0 吗？没有才能交付。
[ ] 我的完成报告诚实吗？没有隐瞒任何失败 / 跳过？
[ ] 我有没有发现新优化机会？写到 OPTIMIZATIONS.md 了吗？
[ ] 我准备好接受用户挑战每一行改动了吗？
```

---

## 第 11 章：与其他文档的导航

```
我现在的状态                  → 我应该读哪份文档
─────────────────────────────────────────────────
刚接到任务                    → AGENTS.md（本文档）
准备写计划，需要规则细节       → ENGINEERING_GUIDE.md
找现成任务认领                → OPTIMIZATIONS.md
理解架构决策（如为什么 9 服务）→ docs/ARCHITECTURE.md
改某个服务的具体代码           → services/<name>/README.md
改契约                       → contracts/README.md
紧急情况                      → ENGINEERING_GUIDE.md 第 14 章「紧急情况处理」
```

---

## 第 12 章：本文档的更新规则

- 任何对本文档的修改必须经用户审核
- 如果你（agent）发现某条 SOP 有问题，按第 7 章「异议机制」走
- 修改后在底部更新版本号 + 变更记录

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-14 | 初版，定义 11 章强制工作流 SOP |
| v1.1 | 2026-06-14 | 新增第 4 章「代码评审与问题闭环」(强制 5 步：/code-review → 登记 QUESTION → 修复 → 二次评审 → 报告)；DoD 加 3 个评审出口；任务 SOP 8.1-8.4 全部加 review 闭环步骤；第 9 章禁止行为加 5 条评审相关；自我提问检查加 4 条评审问题；总章节 11 → 12 |
| v1.2 | 2026-06-14 | 新增 3 个跨服务协作 SOP：8.5「dashboard 发现缺接口」/ 8.6「orchestrator 实现接口」/ 8.7「dashboard 接入已实现接口」；引入 [`services/orchestrator/API_REQUESTS.md`](./services/orchestrator/API_REQUESTS.md) 作为接口需求登记簿（三状态闭环 [ ] → [~] → [x] → [✓]）；原 8.5「写文档」顺延为 8.8；总任务类型 SOP 由 5 个增至 8 个 |
