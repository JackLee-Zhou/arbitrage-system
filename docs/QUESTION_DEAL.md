# 全局问题汇总

> ⚠️ **所有 AI agent 在写代码前必须先读本文档**，了解各模块已知问题和通用约束。
>
> 每个模块的详细问题记录见 `services/<name>/QUESTION.md`。

---

## 按服务索引

| 服务 | QUESTION.md | 待解决 | 已修复 | 状态 |
|---|---|---|---|---|
| ctp_tick | [QUESTION.md](../services/ctp_tick/QUESTION.md) | 4 | 24 | 开发中 |
| trading_agent | [QUESTION.md](../services/trading_agent/QUESTION.md) | 5 | 0 | 待开发 |
| orchestrator | [QUESTION.md](../services/orchestrator/QUESTION.md) | 6 | 14 | 开发中 |
| db_writer | [QUESTION.md](../services/db_writer/QUESTION.md) | 8 | 11 | 开发中 |
| watchdog | [QUESTION.md](../services/watchdog/QUESTION.md) | 6 | 4 | 开发中 |
| llm_scorer | [QUESTION.md](../services/llm_scorer/QUESTION.md) | 0 | 0 | 待开发 |
| intel_collector | [QUESTION.md](../services/intel_collector/QUESTION.md) | 0 | 0 | 待开发 |
| dashboard_web | [QUESTION.md](../services/dashboard_web/QUESTION.md) | 1 | 0 | 设计阶段 |
| stock_tick | [QUESTION.md](../services/stock_tick/QUESTION.md) | 0 | 0 | V2 占位 |

---

## 通用问题（跨服务）

| # | 发现时间 | 描述 | 影响范围 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-13 | go-redis v9.20.1 API 变更：XReadGroupArgs 无 ID 字段，Streams 是 interleaved 格式 `[stream1, id1, stream2, id2]` | 所有 Go 服务 | ✅ 已修复 (pkg/redis/adapter.go) |
| 2 | 2026-06-13 | golangci-lint v2 配置格式不兼容 v1：必须加 `version: "2"` + linters/formatters 分离 + issues → exclusions 迁移 | 所有 Go 服务 | ✅ 已修复 (.golangci.yml) |
| 3 | 2026-06-14 | 🔴 orchestrator ↔ trading_agent gRPC 依赖：orchestrator 当前用 rawCodec（JSON 透传），待 trading_agent 开发时 `make gen` 生成 proto stub 后切换。trading_agent 须实现 SubscribeStateChanges RPC（server stream），orchestrator 端 stream.go 已预留接口 | orchestrator + trading_agent | open |
| 3 | 2026-06-13 | `interface{}` 返回类型导致 `.Close()` 编译错误：Go 不支持对 `interface{}` 直接调用方法 | 所有返回 `interface{}` 的桩代码 | ✅ 已修复 (ctp_tick cmd/main.go) |

---

## 按日期汇总

### 2026-06-13

#### 修复（7 项 — ctp_tick）
| # | 服务 | 描述 | 修复方式 |
|---|---|---|---|
| 1 | ctp_tick | compile: interface{} 无 Close 方法 | 定义 RedisClient struct |
| 2 | ctp_tick | compile: DecayScore 重复定义 | 从 fuser.go 删除 |
| 3 | ctp_tick | lint: w.Write/fmt.Fprintf 返回值未检查 | `_, _ =` 显式忽略 |
| 4 | ctp_tick | lint: gofmt 格式不一致 | gofmt -w 批量格式化 |
| 5 | ctp_tick | Redis 桩代码无真实连接 | 引入 go-redis Client |
| 6 | ctp_tick | 各服务 Redis 代码重复 | 抽取 pkg/redis 共享包 |
| 7 | ctp_tick | 单合约订阅 + 同步落库阻塞 | TD API 全市场 + 异步管道 |

#### 待解决（2 项 — ctp_tick）
| # | 服务 | 描述 | 阻塞因素 |
|---|---|---|---|
| 1 | ctp_tick | cgo C struct 字段提取未实现 | 需 Linux 容器 + openctp 头文件 |
| 2 | ctp_tick | ReqUserLogin C 结构体构建未完成 | 需 openctp/ctp-go 头文件 |

### 2026-06-14

#### 修复（10 项 — db_writer）

| # | 服务 | 描述 | 修复方式 |
|---|---|---|---|
| 1 | db_writer | 🔴 DLQ TTL 用 SET 操作 Stream key 返回 WRONGTYPE | pkg/redis 新增 Expire 接口，dlq.go 改用 Expire |
| 2 | db_writer | 🔴 XACK 在 InfluxDB 写入前执行，写入失败数据丢失 | 重构消息处理：缓存 ID，写入成功/DLQ 成功后才 XACK |
| 3 | db_writer | 🔴 influxWriter.Close() 被 defer + 显式双重调用 | 删除 main.go 显式 Close() |
| 4 | db_writer | 🟡 ctx 取消后 flushRemaining 用已取消 ctx | 改为 context.Background() |
| 5 | db_writer | 🟡 XGroupCreate 缺少 MKSTREAM | 改为 XGroupCreateMkStream |
| 6 | db_writer | 🟡 /healthz 始终返回 200，unhealthy 仍路由流量 | 根据状态设置 200/503 |
| 7 | db_writer | 🟡 重试 off-by-one：MaxRetries=3 实际只用 2 次 | 修正 ShouldRetry 参数 |
| 8 | db_writer | 🟢 XGroupCreate 错误完全吞掉 | 非 BUSYGROUP 错误记录日志 |
| 9 | db_writer | 🟢 gofmt 格式不一致（5 文件） | gofmt -w 格式化 |
| 10 | db_writer | 🟢 lint: errcheck — Body.Close/fmt.Fprint 返回值 | defer func / `_, _ =` |

#### 待解决（7 项 — db_writer）

| # | 服务 | 描述 | 阻塞因素 |
|---|---|---|---|
| 1 | db_writer | DLQ 写入逐条 XADD，大量失败时性能差 | 低优先级 |
| 2 | db_writer | ConsumerLag 指标未实际采集 | 需 XINFO GROUPS |
| 3 | db_writer | getEnv 模式三处重复（ctp_tick/watchdog/db_writer） | 待抽取 pkg/env |
| 4 | db_writer | Stream→Bucket 映射硬编码 | 设计决策 |
| 5 | db_writer | InfluxDB HTTP 超时未显式设置 | 低优先级 |
| 6 | db_writer | processMessages 持锁期间网络调用 | 当前单 goroutine 安全 |
| 7 | db_writer | go.mod 版本 1.26.4 vs CLAUDE.md 1.22 | 待统一 |

#### 基础设施

| # | 描述 | 影响范围 | 状态 |
|---|---|---|---|
| 1 | pkg/registry 服务注册/心跳包创建 | 全部 Go 服务 | ✅ 已交付 (OPT-002 Phase 1) |
| 2 | pkg/redis 新增 XLen/XGroupCreate/Expire 方法 | db_writer | ✅ 已交付 |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版，汇总 ctp_tick 开发过程 7 个已修复 + 2 个待解决 + 3 个通用问题 |
| v1.1 | 2026-06-14 | db_writer 首轮开发：10 修复 + 7 待解决；db_writer 状态更新为"开发中"；pkg/registry + pkg/redis 扩展交付 |
| v1.2 | 2026-06-14 | dashboard_web 设计阶段：登记「紧急平仓仅在持仓页」的设计风险（accepted） |
