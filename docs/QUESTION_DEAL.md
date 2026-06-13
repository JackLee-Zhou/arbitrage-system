# 全局问题汇总

> ⚠️ **所有 AI agent 在写代码前必须先读本文档**，了解各模块已知问题和通用约束。
>
> 每个模块的详细问题记录见 `services/<name>/QUESTION.md`。

---

## 按服务索引

| 服务 | QUESTION.md | 待解决 | 已修复 | 状态 |
|---|---|---|---|---|
| ctp_tick | [QUESTION.md](../services/ctp_tick/QUESTION.md) | 2 | 7 | 开发中 |
| trading_agent | [QUESTION.md](../services/trading_agent/QUESTION.md) | 0 | 0 | 待开发 |
| orchestrator | [QUESTION.md](../services/orchestrator/QUESTION.md) | 0 | 0 | 待开发 |
| db_writer | [QUESTION.md](../services/db_writer/QUESTION.md) | 0 | 0 | 待开发 |
| watchdog | [QUESTION.md](../services/watchdog/QUESTION.md) | 0 | 0 | 待开发 |
| llm_scorer | [QUESTION.md](../services/llm_scorer/QUESTION.md) | 0 | 0 | 待开发 |
| intel_collector | [QUESTION.md](../services/intel_collector/QUESTION.md) | 0 | 0 | 待开发 |
| dashboard_web | [QUESTION.md](../services/dashboard_web/QUESTION.md) | 0 | 0 | 待开发 |
| stock_tick | [QUESTION.md](../services/stock_tick/QUESTION.md) | 0 | 0 | V2 占位 |

---

## 通用问题（跨服务）

| # | 发现时间 | 描述 | 影响范围 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-13 | go-redis v9.20.1 API 变更：XReadGroupArgs 无 ID 字段，Streams 是 interleaved 格式 `[stream1, id1, stream2, id2]` | 所有 Go 服务 | ✅ 已修复 (pkg/redis/adapter.go) |
| 2 | 2026-06-13 | golangci-lint v2 配置格式不兼容 v1：必须加 `version: "2"` + linters/formatters 分离 + issues → exclusions 迁移 | 所有 Go 服务 | ✅ 已修复 (.golangci.yml) |
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

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版，汇总 ctp_tick 开发过程 7 个已修复 + 2 个待解决 + 3 个通用问题 |
