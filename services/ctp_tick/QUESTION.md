# ctp_tick — 问题记录

> 记录本模块开发过程中发现的问题和修复方式。每次修复后同步更新 `docs/QUESTION_DEAL.md`。

---

## 当前待解决问题

| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-13 | cgo C struct 字段提取未实现（spi.go cFieldToDepthMarketData）— 需 Linux 容器环境 | 🟡 | open |
| 2 | 2026-06-13 | ReqUserLogin C 结构体构建未完成（md.go login 方法）— 需 openctp/ctp-go 头文件 | 🟡 | open |
| 3 | 2026-06-13 | 🔴 **Goroutine 泄露**: flushBatch() 每次 new goroutine，Close() 不等待它们退出（writer.go:203） | 🔴 | open |
| 4 | 2026-06-13 | 🔴 **Goroutine 泄露**: reconnect 中 time.After goroutine 无法取消，Close() 后仍可能调用 attemptReconnect()（reconnect.go:53） | 🔴 | open |
| 5 | 2026-06-13 | 🔴 **Data Race**: attemptReconnect() 无锁替换 connected/loggedIn channel，与 CTP 回调线程并发访问（reconnect.go:76-77） | 🔴 | open |
| 6 | 2026-06-13 | 🟡 **内存泄露**: spi.go 全局 apiRegistry map 只增不删，Close() 无清理（spi.go:50-58） | 🟡 | open |
| 7 | 2026-06-13 | 🟡 **数据丢失**: writerLoop 关闭时不排空 tickCh，通道内残余 Tick 直接丢弃（writer.go:121-135） | 🟡 | open |
| 8 | 2026-06-13 | 🟡 **阻塞风险**: sendFocus() 同步 XADD 阻塞 writerLoop，Redis 慢时拖慢整条管道（writer.go:214） | 🟡 | open |
| 9 | 2026-06-13 | 🟡 **端口暴露**: HTTP Server 绑定 0.0.0.0:8081，无 ReadTimeout/WriteTimeout（Slowloris 风险）（cmd/main.go:74） | 🟡 | open |
| 10 | 2026-06-13 | 🟡 **数据丢失**: convertToRaw() 仅映射 Bid/Ask 第1档，丢弃 Bid2-5/Ask2-5 5档深度数据（cmd/main.go:148） | 🟡 | open |
| 11 | 2026-06-13 | 🟡 **资源泄露**: md.go Close() 中 C.Release 被注释，C 侧 API 资源未释放（md.go:136） | 🟡 | open |
| 12 | 2026-06-13 | 🟢 **死代码**: reconnectCh 字段声明但从未使用（md.go:50） | 🟢 | open |
| 13 | 2026-06-13 | 🟢 **代码异味**: domain/normalizer.go:205 `var _ = math.MaxFloat64` 抑制未使用 import | 🟢 | open |
| 14 | 2026-06-13 | 🟢 **常量重复**: MaxValidPrice/MaxValidVolume 在 domain/normalizer.go 和 adapters/ctp/types.go 重复定义 | 🟢 | open |
| 15 | 2026-06-13 | 🟢 **安全**: Dockerfile 无 USER 指令，容器以 root 运行 | 🟢 | open |
| 16 | 2026-06-13 | 🟢 **健壮性**: MockMdAPI.Close() 缺少 sync.Once 保护，重复调用 close(stopCh) 会 panic（md_mock.go:83） | 🟢 | open |
| 17 | 2026-06-13 | 🟢 **健壮性**: 优雅退出 5s 超时后无 os.Exit(1) 强制退出兜底（cmd/main.go:135） | 🟢 | open |

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
| v1.1 | 2026-06-13 | 安全性专项审查：发现 15 个新问题（3🔴 + 6🟡 + 6🟢），涵盖 goroutine 泄露、data race、内存泄露、端口暴露等 |
