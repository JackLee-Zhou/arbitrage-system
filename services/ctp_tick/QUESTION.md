# ctp_tick — 问题记录

> 记录本模块开发过程中发现的问题和修复方式。每次修复后同步更新 `docs/QUESTION_DEAL.md`。

---

## 当前待解决问题

| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-13 | cgo C struct 字段提取未实现（spi.go cFieldToDepthMarketData）— 需 Linux 容器环境 | 🟡 | open |
| 2 | 2026-06-13 | ReqUserLogin C 结构体构建未完成（md.go login 方法）— 需 openctp/ctp-go 头文件 | 🟡 | open |

---

## 已修复问题

| # | 发现时间 | 修复时间 | 描述 | 修复方式 | 版本 |
|---|---|---|---|---|---|
| 1 | 2026-06-13 | 2026-06-13 | cmd/main.go 中 connectWithRetry 返回 interface{} 导致 `.Close()` 编译错误 | 定义 `RedisClient` struct 替代 `interface{}` 返回类型 | v0.1.0 |
| 2 | 2026-06-13 | 2026-06-13 | trading_agent domain/decision/fuser.go 和 decay.go 中 DecayScore 函数重复定义 | 从 fuser.go 移除 DecayScore，保留 decay.go 中的定义 | v0.1.0 |
| 3 | 2026-06-13 | 2026-06-13 | errcheck lint：w.Write / fmt.Fprintf / redisClient.Close 返回值未检查 | 代码中 `_, _ =` 显式忽略 + `log.Fatal(http.ListenAndServe(...))` 处理 | v0.1.1 |
| 4 | 2026-06-13 | 2026-06-13 | gofmt 格式不一致（struct 字段对齐） | `gofmt -w` 批量格式化所有 .go 文件 | v0.1.1 |
| 5 | 2026-06-13 | 2026-06-13 | GoRedisAdapter XAdd/Ping/Close 是桩代码，无实际 Redis 连接 | 引入 `*redis.Client` + `redis.XAddArgs{MaxLen, Approx}` 真实实现 | v0.1.2 |
| 6 | 2026-06-13 | 2026-06-13 | 各 Go 服务各自定义 RedisClient 接口 + GoRedisAdapter，代码重复 | 抽取 `pkg/redis/` 共享包，统一接口 + NewAdapter 入口 | v0.2.0 |
| 7 | 2026-06-13 | 2026-06-13 | 仅硬编码订阅 MA2509，不查询全市场合约；落库同步阻塞 CTP 回调 | 新增 TD API 全市场查询 + 异步 writerLoop（channel + goroutine）+ 批量 write:tick（100条/1s） | v0.2.1 |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版，记录 ctp_tick 开发过程中的 7 个已修复问题 + 2 个待解决问题 |
