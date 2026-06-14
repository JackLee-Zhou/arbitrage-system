# ctp_tick — 问题记录

> 记录本模块开发过程中发现的问题和修复方式。每次修复后同步更新 `docs/QUESTION_DEAL.md`。

---

## 当前待解决问题

| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-13 | cgo C struct 字段提取未实现（spi.go cFieldToDepthMarketData）— 需 Linux 容器环境 | 🟡 | open |
| 2 | 2026-06-13 | ReqUserLogin C 结构体构建未完成（md.go login 方法）— 需 openctp/ctp-go 头文件 | 🟡 | open |
| 3 | 2026-06-14 | Mock 仅生成字母序首个 symbol（MA509）数据，与 `SUBSCRIBE_SYMBOLS=MA2509` 不匹配，导致 `tick:MA2509` 流始终为空 | 🟡 | open |
| 4 | 2026-06-14 | `ctp_connection_status` gauge 在 Mock 模式下未更新，始终为 0（disconnected），但健康检查报告 ctp:ok | 🟢 | open |

---

## 已修复问题

| # | 发现时间 | 修复时间 | 描述 | 修复方式 | 版本 |
|---|---|---|---|---|---|
| 1 | 2026-06-13 | 2026-06-13 | compile: interface{} 返回 Close() 错误 | 定义 RedisClient struct | v0.1.0 |
| 2 | 2026-06-13 | 2026-06-13 | compile: DecayScore 函数重复定义 | 从 fuser.go 删除 | v0.1.0 |
| 3 | 2026-06-13 | 2026-06-13 | lint: w.Write/fmt.Fprintf 返回值未检查 | `_, _ =` 显式忽略 | v0.1.1 |
| 4 | 2026-06-13 | 2026-06-13 | lint: gofmt 格式不一致 | gofmt -w 批量格式化 | v0.1.1 |
| 5 | 2026-06-13 | 2026-06-13 | Redis 桩代码无真实连接 | 引入 go-redis Client | v0.1.2 |
| 6 | 2026-06-13 | 2026-06-13 | 各服务 Redis 代码重复 | 抽取 pkg/redis 共享包 | v0.2.0 |
| 7 | 2026-06-13 | 2026-06-13 | 单合约订阅 + 落库同步阻塞 | TD API 全市场 + 异步管道 | v0.2.1 |
| 8 | 2026-06-13 | 2026-06-13 | 🔴 flushBatch goroutine 无等待 | flushWg.WaitGroup 追踪 + Close 等待 | v0.2.2 |
| 9 | 2026-06-13 | 2026-06-13 | 🔴 time.After goroutine 无法取消 | AfterFunc + Stop + reconnectTimer 字段 | v0.2.2 |
| 10 | 2026-06-13 | 2026-06-13 | 🔴 channel 无锁替换 data race | attemptReconnect 加 mu.Lock 保护 | v0.2.2 |
| 11 | 2026-06-13 | 2026-06-13 | 🟡 apiRegistry map 内存泄露 | unregisterAPI + cleanupRegistryByAPI | v0.2.2 |
| 12 | 2026-06-13 | 2026-06-13 | 🟡 writerLoop 关闭丢弃 tickCh | return 前 drain tickCh + 最后 flush | v0.2.2 |
| 13 | 2026-06-13 | 2026-06-13 | 🟡 sendFocus 同步阻塞 writerLoop | 改为 go w.sendFocus 异步 | v0.2.2 |
| 14 | 2026-06-13 | 2026-06-13 | 🟡 HTTP Server 无 Timeout | Read/Write/IdleTimeout 10s/10s/120s | v0.2.2 |
| 15 | 2026-06-13 | 2026-06-13 | 🟡 5 档深度丢弃 Bid2-5/Ask2-5 | RawMarketData 增加 16 字段 + convertToRaw 全映射 | v0.2.2 |
| 16 | 2026-06-13 | 2026-06-13 | 🟡 C.Release 被注释（虽需 Linux） | 文档注释说明 | v0.2.2 |
| 17 | 2026-06-13 | 2026-06-13 | 🟢 reconnectCh 死代码 | 删除字段 | v0.2.2 |
| 18 | 2026-06-13 | 2026-06-13 | 🟢 math.MaxFloat64 hack | 删除 + 移除未使用的 math import | v0.2.2 |
| 19 | 2026-06-13 | 2026-06-13 | 🟢 常量重复定义 | types.go 删除重复，统一由 domain 定义 | v0.2.2 |
| 20 | 2026-06-13 | 2026-06-13 | 🟢 Dockerfile 无 USER | 添加 USER 1000:1000 | v0.2.2 |
| 21 | 2026-06-13 | 2026-06-13 | 🟢 MockMdAPI.Close 缺 sync.Once | closeOnce.Do 保护 | v0.2.2 |
| 22 | 2026-06-13 | 2026-06-13 | 🟢 Shutdown 5s 超时无兜底 | 超时后 log force exit | v0.2.2 |
| 23 | 2026-06-14 | 2026-06-14 | 🔴 `parseTimestamp` 用 `time.Parse`（UTC）解析 CTP 本地时间（CST），导致 InfluxDB 时间戳偏移 +8h | 改用 `time.ParseInLocation` + `Asia/Shanghai` | v0.2.3 |
| 24 | 2026-06-14 | 2026-06-14 | 🟡 `data` 字段写入 JSON，db_writer 期望 Line Protocol，数据格式不兼容导致 InfluxDB 写入失败 | `formatLineProtocol()` 函数生成 LP 格式，`json_data` 保留 JSON | v0.2.3 |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版，7 个已修复 + 2 个待解决 |
| v1.1 | 2026-06-13 | 安全性审查：新增 15 个问题（3🔴 + 6🟡 + 6🟢） |
| v1.2 | 2026-06-13 | 15 个安全问题全部修复（##8-##22），仅剩 2 个需要 Linux cgo 环境的待解决 |
| v1.3 | 2026-06-14 | 通讯调试：修复时间戳偏移 + 数据格式桥接，新增 2 个待解决（Mock symbol 不匹配 + gauge 未更新） |
