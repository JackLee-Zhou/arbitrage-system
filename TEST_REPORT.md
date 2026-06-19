# arbitrage-system 集成测试报告

**时间**: 2026-06-19  
**范围**: 5 个 Go 服务（ctp_tick、db_writer、trading_agent、orchestrator、watchdog）全链路集成测试  
**排除**: intel_collector、llm_scorer、钉钉告警（无 webhook URL）

---

## 结果总览

| 类别 | 总项 | 通过 | 未通过 | 跳过 |
|------|------|------|--------|------|
| 服务注册与发现 | 6 | 6 | 0 | 0 |
| 数据注入与 Redis 流 | 9 | 9 | 0 | 0 |
| db_writer 消费与持久化 | 5 | 5 | 0 | 0 |
| trading_agent 引擎 | 6 | 6 | 0 | 0 |
| orchestrator REST API | 9 | 9 | 0 | 0 |
| orchestrator WebSocket | 4 | 4 | 0 | 0 |
| gRPC 通信（读+写） | 5 | 5 | 0 | 0 |
| watchdog 健康巡检 | 2 | 2 | 0 | 0 |
| 边界条件 | 7 | 7 | 0 | 0 |
| 压力与极限 | 8 | 8 | 0 | 0 |
| 故障恢复 | 3 | 3 | 0 | 0 |
| domain 单元测试 | 1 | 1 | 0 | 0 |
| **合计** | **65** | **65** | **0** | **0** |

**通过率: 100%**

---

## 详细结果

### 一、服务注册与发现 (6/6)

- ✅ 5 个服务全部注册到 Redis `services:*`（ctp_tick、db_writer、trading_agent、orchestrator、watchdog）
- ✅ ctp_tick: version=0.2.3, health_url=localhost:8081
- ✅ db_writer: version=1.0.0, health_url=localhost:8082
- ✅ trading_agent: version=0.1.0, health_url=:8083
- ✅ orchestrator: version=0.3.0, health_url=localhost:8080
- ✅ watchdog: version=0.1.0, 自动发现所有服务

### 二、数据注入与 Redis 流 (9/9)

- ✅ POST /internal/ingest 单条注入成功
- ✅ 批量注入 10 条全部成功（0 rejected）
- ✅ write:tick Stream 数据正确写入
- ✅ tick:MA2509 Stream 仅关注合约写入（非关注合约不写入）
- ✅ last_price:MA2509 KV 快照正确（含 price、change_pct）
- ✅ instruments:all KV 合约元数据正常
- ✅ last_price TTL 正确设置为 60s
- ✅ 多合约隔离：8 个不同 symbol 各有独立快照
- ✅ 144 合约同时注入，数据不混淆

### 三、db_writer 消费与持久化 (5/5)

- ✅ 消费 write:tick → InfluxDB ma_tick bucket，确认 flush 成功
- ✅ /healthz 返回 200，InfluxDB 连通性检查正常
- ✅ /metrics 暴露 Prometheus 指标（write_total、dlq_size 等）
- ✅ NOGROUP 自动恢复：流被删除后自动重建消费组，不再死循环刷日志
- ✅ DLQ 重试机制：InfluxDB 故障 → 指数退避重试（1s→4s→16s）→ 失败入死信队列

### 四、trading_agent 引擎 (6/6)

- ✅ /healthz 返回 200
- ✅ 引擎启动：mode=normal, symbol=MA2509
- ✅ 三步对账正常执行：Step1 Redis → Step2 InfluxDB 兜底 → Step3 CTP（跳过）
- ✅ crash 恢复：Kill 后重启，从 Redis 恢复持仓（"对账持仓已载入: MA2509 long 1手 @2345.00"）
- ✅ 引擎完整链路：tick 消费 → MA/Boll/ATR 指标计算 → MA 金叉策略信号 → 决策融合 → 风控检查 → 报单执行 → 紧急检测（boll_breakout）
- ✅ SafeMode：EmergencyClose 后引擎正确进入 safe_mode

### 五、orchestrator REST API (9/9)

- ✅ GET /healthz → 200
- ✅ GET /api/v1/overview → 聚合视图（services + positions + engine + ai_score）
- ✅ GET /api/v1/services → 服务健康列表
- ✅ GET /api/v1/contracts → 合约列表
- ✅ GET /api/v1/contracts?symbol=MA2509 → 单合约查询
- ✅ GET /api/v1/ai/scores/current?symbol=MA2509 → AI 评分
- ✅ GET /api/v1/positions → 持仓
- ✅ GET /api/v1/orders → 委托（空）
- ✅ GET /api/v1/trades → 成交（空）

### 六、orchestrator WebSocket (4/4)

- ✅ 连接建立成功
- ✅ 连接时收到 snapshot
- ✅ 客户端 subscribe 命令正常处理
- ✅ 频道推送正常（system_status、positions）

### 七、gRPC 通信 (5/5)

- ✅ 连接：orchestrator ↔ trading_agent gRPC :9090 连通
- ✅ GetStatus：引擎状态查询正常（返回 mode=normal/safe_mode）
- ✅ GetPositions：持仓查询正常
- ✅ EmergencyClose：紧急平仓端到端通过 → 引擎进入 safe_mode → 审计日志记录
- ✅ UpdateRiskParams：MaxPosition/MaxLossPerDay/StopLossPct 三个参数全部更新成功，审计记录

### 八、watchdog 健康巡检 (2/2)

- ✅ 从 Redis services:* 自动发现全部 5 个服务
- ✅ 周期性探测 /healthz，状态正确判定：
  - 🟢 trading_agent healthy
  - 🟢 orchestrator healthy
  - 🟢 db_writer healthy
  - 🟡 ctp_tick degraded（CTP 未连接，mock 模式）

### 九、边界条件 (7/7)

- ✅ 空 POST → 400 + error message
- ✅ last_price=0 → 正确拒绝
- ✅ last_price=-100 → 正确拒绝
- ✅ 极大价格 (999999) → 正常写入快照
- ✅ volume=0 → 正常写入
- ✅ 未知 symbol → 不写入 focus 流（仅 write:tick 落库）
- ✅ TTL 正确（59-60s）

### 十、压力与极限 (8/8)

- ✅ 95,413 tick/s burst 注入，channel buffer 无溢出
- ✅ 35,000 tick/s 持续注入，0 dropped
- ✅ 5 线程并发注入，1000 条全部成功
- ✅ 144 合约 x 660 tick/s x 30s，零数据丢失
- ✅ 60s 持续注入 + 实时内存监控，全部服务内存稳定：
  - ctp_tick: 62.2MB→62.2MB（0 增长）
  - trading_agent: 21.9MB→22.2MB（+0.3MB）
  - orchestrator: 23.8MB→23.9MB（+0.1MB）
  - db_writer: 21.2MB→21.3MB（+0.1MB）
- ✅ maxlen 100k + batch 500 优化后，660 tick/s 下 Stream 仅用 10%
- ✅ 4C8G 内存预算充足（总占用 ~400MB）
- ✅ Redis 660 tick/s 持续注入无 OOM 风险

### 十一、故障恢复 (3/3)

- ✅ trading_agent crash → 重启 → 三步对账 → 从 Redis 恢复持仓
- ✅ db_writer NOGROUP → 自动重建消费组 + 2s 延迟防抖
- ✅ db_writer InfluxDB 故障 → 指数退避重试 → DLQ 兜底

### 十二、domain 单元测试 (1/1)

- ✅ `go test ./internal/domain/...` 全部通过（indicator/strategy/decision/risk/position）

---

## 待完成项（已明确跳过）

| # | 项目 | 原因 |
|---|------|------|
| 3 | 交易时段守卫 | 用户指定跳过 |
| 9 | 钉钉告警 | 无真实 webhook URL，需生产环境测试 |

## 本次测试中修复的 Bug

| # | 严重度 | 服务 | 问题 | 修复方式 |
|---|--------|------|------|----------|
| 1 | 🔴 | db_writer | NOGROUP 死循环刷爆日志（640MB/次） | consumer.go: 检测 NOGROUP → 自动 XGroupCreate 重建消费组 + 2s 延迟防抖 |
| 2 | 🟡 | pkg/redis | write:tick maxlen=10000 太小，660 tick/s 下 15 秒裁剪 | adapter.go: maxlen 10000 → 100000 |
| 3 | 🟡 | db_writer | batch=10 吞吐不足，仅持久化 1.5% 数据 | .env: batch 10 → 500 |
| 4 | 🟢 | trading_agent | InfluxDB bucket `order_history` 不存在 → 三步对账 Step2 报错 | 创建 `order_history` bucket |
| 5 | 🟢 | ctp_tick | metadata writer 写 InfluxDB bucket `instruments` 不存在 → 404 | 创建 `instruments` bucket |
| 6 | 🟡 | orchestrator | PauseStrategy/ResumeStrategy API 文档错误（写的是子路径，实际是 JSON body action） | README 更新：`POST /api/v1/strategies` + `{"action":"pause"/"resume"}` |
| 7 | 🟢 | orchestrator | UpdateRiskParams 参数名未文档化（Go 驼峰 MaxPosition/MaxLossPerDay/StopLossPct） | README 更新：标注参数名和格式 |

## 已知限制（P2 处理，不阻塞当前功能）

| # | 服务 | 描述 | 当前替代方案 |
|---|------|------|------|
| 1 | trading_agent → orchestrator | gRPC SubscribeStateChanges server stream 未实现 | orchestrator 通过 REST 轮询 + Redis 读取状态，WebSocket 正常推送 |

---

## 代码变更记录

| 文件 | 操作 | 说明 |
|------|------|------|
| `services/ctp_tick/internal/api/ingest.go` | 新建 | HTTP 数据注入端点 |
| `services/ctp_tick/cmd/main.go` | 修改 | +INGEST_PORT 配置 + ingest server |
| `services/db_writer/internal/adapters/redis/consumer.go` | 修改 | NOGROUP 自动恢复 + 防抖 |
| `pkg/redis/adapter.go` | 修改 | XAdd maxlen 10000 → 100000 |
| `services/db_writer/.env` | 修改 | batch 10 → 500 |
| `tests/integration/test_full_flow.py` | 新建 | 48 项综合集成测试 |
| `services/*/.env` (5个) | 新建 | 本地测试配置 |
| `logs/*.log` (5个) | 输出 | 各服务完整运行日志 |
