# watchdog — 问题记录

> 记录本模块开发过程中发现的问题和修复方式。每次修复后同步更新此文件。

---

## 当前待解决问题

| # | 发现时间 | 描述 | 严重度 | 状态 |
|---|---|---|---|---|
| 1 | 2026-06-14 | 🔴 延迟检测盲区：健康检查仅判定 healthy/unhealthy，无延迟阈值告警。SIGSTOP 5s 仅被记录为 1819ms 延迟而非故障 | 🔴 | open |
| 2 | 2026-06-14 | 🟡 内存检测使用 `runtime.ReadMemStats`（Go 进程级 ~5%），而非宿主机真实内存，三级告警形同虚设 | 🟡 | open |
| 3 | 2026-06-14 | 🟡 重试逻辑可能掩盖瞬时故障：`healthz_client.go` 重试间隔 500ms，故障在此窗口内恢复则完全不可见 | 🟡 | open |
| 4 | 2026-06-14 | 🟡 告警无 fallback：未配置 `DINGTALK_WEBHOOK_URL` 时告警静默丢弃，无本地持久化或替代通知渠道 | 🟡 | open |
| 5 | 2026-06-14 | 🟢 `adapters/disk_monitor.go` 和 `memory_monitor.go` 仍为桩代码（仅 TODO 注释），实际逻辑全部写在 main.go | 🟢 | open |
| 6 | 2026-06-14 | 🟢 磁盘告警 60% 阈值偏低（正常开发环境已达 57.6%），建议调整为 75%/85%/95% | 🟢 | open |

---

## 已修复问题

| # | 发现时间 | 修复时间 | 描述 | 修复方式 | 版本 |
|---|---|---|---|---|---|
| 1 | 2026-06-14 | 2026-06-14 | 🔴 健康检查逻辑为空桩（healthCheckLoop 仅打印日志后返回） | 实现 `healthz_client.go` HTTP 探测 + 失败重试 + 状态变更告警 | v0.1.0 |
| 2 | 2026-06-14 | 2026-06-14 | 🔴 钉钉告警适配器为空桩（dingtalk.go 仅 TODO 注释） | 实现 Markdown 格式 Webhook 推送 + 去重逻辑 | v0.1.0 |
| 3 | 2026-06-14 | 2026-06-14 | 🟡 `SCRAPE_INTERVAL` 环境变量未接入（.env.example 定义但代码硬编码 5s） | `loadConfig()` 中读取 `SCRAPE_INTERVAL` + `PROBE_TIMEOUT`，支持 `time.ParseDuration` | v0.1.0 |
| 4 | 2026-06-14 | 2026-06-14 | 🟡 diskMonitorLoop / memoryMonitorLoop 为空桩，磁盘/内存监控未实现 | 实现 `checkDiskUsage`（syscall.Statfs）+ `checkMemoryUsage`（runtime.ReadMemStats）+ 三级告警逻辑 | v0.1.0 |

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 占位初版 |
| v1.1 | 2026-06-14 | 首轮开发：实现健康检查 + 钉钉告警 + 磁盘/内存监控，修复 4 个问题；6 个待解决 |
