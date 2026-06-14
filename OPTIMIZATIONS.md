# OPTIMIZATIONS.md — 全局优化清单

> **本文档面向其他 AI agent**（Claude Code / Cursor / 其他协作 agent）。
> 这里记录**已识别但尚未执行**的代码优化项 + 重构需求。
> 处理完一项 → 在本文档勾掉 + 在底部加变更记录 + 同步更新对应的服务 README。

---

## 阅读顺序

任何 agent 来处理本文档的任一项前，**必须**先读：

1. [`/ENGINEERING_GUIDE.md`](./ENGINEERING_GUIDE.md) — 工程规则（含 Rule of Three 等）
2. [`/docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) — 架构方案
3. [`/contracts/README.md`](./contracts/README.md) — 跨服务契约
4. 你要改的具体服务的 README

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

## 三、其他已识别的优化（开放区）

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

## 四、Agent 处理流程

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

## 五、变更记录

| 日期 | 任务 ID | Agent | 改动摘要 | PR / commit |
|---|---|---|---|---|
| 2026-06-13 | -- | claude-code | 创建 OPTIMIZATIONS.md 初版，登记 OPT-001 ~ OPT-004 + OPT-101 | -- |
| 2026-06-14 | OPT-001-a~d | claude-code | 完成 Redis 重试逻辑抽取：新增 pkg/redis/retry.go + retry_test.go（11 测试全部通过），Mock 加失败注入，ctp_tick 改用 XAddWithRetry | -- |

---

## 六、不接受的优化（黑名单）

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
| v1.1 | 2026-06-14 | OPT-001 完成：重试逻辑抽取到 pkg/redis/retry.go，ctp_tick 重构完成 |
