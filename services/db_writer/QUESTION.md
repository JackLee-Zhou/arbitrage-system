# db_writer — 问题记录

> 记录本模块开发过程中发现的问题和修复方式。每次修复后同步更新此文件。

---

## 当前待解决问题

| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-14 | DLQ 写入逐条 XADD（无 Pipeline），大量失败时性能差 | 🟢 | open |
| 2 | 2026-06-14 | `ConsumerLag` 指标初始化后从未更新（需 XINFO GROUPS 定期采集） | 🟢 | open |
| 3 | 2026-06-14 | `getEnv`/`getEnvInt`/`getEnvDuration` 在 ctp_tick/watchdog/db_writer 三处重复 | 🟢 | open |
| 4 | 2026-06-14 | Stream→Bucket 映射硬编码在 config.go，添加新 stream 需改代码 | 🟢 | open |
| 5 | 2026-06-14 | InfluxDB HTTP 超时依赖默认值，未显式设置 `SetHTTPRequestTimeout` | 🟢 | open |
| 6 | 2026-06-14 | `processMessages` 持锁期间发起 XACK 网络调用（当前单 goroutine 安全，后续并发路径有风险） | 🟢 | open |
| 7 | 2026-06-14 | go.mod 版本 `1.26.4` 与 CLAUDE.md 声明的 `Go 1.22` 不一致 | 🟢 | open |

---

## 已修复问题

| # | 发现时间 | 修复时间 | 描述 | 修复方式 | 版本 |
|---|---|---|---|---|---|
| 1 | 2026-06-14 | 2026-06-14 | 🔴 DLQ TTL 用 `SET` 命令操作 Stream key，返回 WRONGTYPE，死信数据永不过期 | `pkg/redis` 接口新增 `Expire` 方法，dlq.go 改用 `client.Expire` | v1.0.0 |
| 2 | 2026-06-14 | 2026-06-14 | 🔴 XACK 在 InfluxDB 写入前执行（at-most-once 语义），写入失败时数据丢失 | 重构 `processMessages`/`flushRecords`：缓存消息 ID，写入成功或 DLQ 成功后才 XACK | v1.0.0 |
| 3 | 2026-06-14 | 2026-06-14 | 🔴 `influxWriter.Close()` 被 defer 和显式调用双重执行 | 删除 main.go 中的显式 `Close()` 调用 | v1.0.0 |
| 4 | 2026-06-14 | 2026-06-14 | 🟡 ctx 取消后 `flushRemaining` 使用已取消 ctx，buffer 数据必然丢失 | 改为 `context.Background()` | v1.0.0 |
| 5 | 2026-06-14 | 2026-06-14 | 🟡 `XGroupCreate` 缺少 MKSTREAM，上游未写入时消费组创建失败 | 改为 `XGroupCreateMkStream` | v1.0.0 |
| 6 | 2026-06-14 | 2026-06-14 | 🟡 `/healthz` 始终返回 HTTP 200，unhealthy 时负载均衡器仍路由流量 | 根据 `resp.Status` 设置状态码：healthy→200, unhealthy→503 | v1.0.0 |
| 7 | 2026-06-14 | 2026-06-14 | 🟡 重试逻辑 off-by-one：`MaxRetries=3` 实际只重试 2 次 | 修正 `ShouldRetry(attempt+1)` 为 `ShouldRetry(attempt)` | v1.0.0 |
| 8 | 2026-06-14 | 2026-06-14 | 🟢 `XGroupCreate` 错误被 `_` 完全吞掉（包括网络错误和权限错误） | 非 BUSYGROUP 错误记录日志 | v1.0.0 |
| 9 | 2026-06-14 | 2026-06-14 | 🟢 gofmt 格式不一致（dingtalk.go, consumer.go, dlq.go, main.go, healthz.go） | `gofmt -w` 批量格式化 | v1.0.0 |
| 10 | 2026-06-14 | 2026-06-14 | 🟢 lint: `errcheck` — `resp.Body.Close` 和 `fmt.Fprint` 返回值未检查 | defer func + `_, _ =` 显式忽略 | v1.0.0 |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 占位初版 |
| v1.1 | 2026-06-14 | 首轮开发：10 个问题（3🔴 + 4🟡 + 3🟢）已修复，7 个待解决 |
