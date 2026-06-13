# services/stock_tick — 股票行情订阅服务（V2 占位）

> 本目录仅作占位。V1 阶段**不实现**，预留给未来扩展股票模块。

---

## 状态

🟡 **V2 占位** — 不在当前实施路线（P1-P3）内。

实施时机：参考 `docs/ARCHITECTURE.md` 中 V2 / 后续阶段。**最早 P3 完成后**才考虑。

---

## 规划职责（未来）

### 做什么 ✅（V2 加股票模块时）
- 订阅 A 股行情（化工板块标的，如万华化学、华峰化学等）
- 对标 ctp_tick 的架构：`domain/`（标准化）+ `adapters/`（行情接口）
- 写入 Redis Stream `tick:<stock_symbol>`

### 接入选项（V2 调研结果）

| 选项 | 难度 | 门槛 |
|---|---|---|
| **QMT / miniQMT** | 中 | 资产 ≥ 50 万开户（需联系券商开通） |
| **easytrader** | 低 | 模拟点击 Web 界面，免费但不稳 |
| **ptrade** | 中 | 恒生 + 国金，资产门槛 |
| **火币资本市场** | 低 | 个人可开 |

⚠️ QMT 仅 Windows 可跑，需另购 Windows ECS 跳板（ARCHITECTURE.md v1.9 已分析）。

---

## 与其他服务的关系（未来）

```
stock_tick → Redis Stream tick:<symbol>
                          ↓
                  trading_agent
                          ↓
                  Broker interface (新增 stock 实现)
```

trading_agent.adapters.broker.interface.go 已为 stock 接入预留扩展点。
**业务逻辑（domain/strategy/）零修改**。

---

## 当前不要做什么

- ❌ 不要在 P1-P3 阶段往这里写代码
- ❌ 不要为了"先占位"建 cmd/main.go 之类的文件
- ❌ 不要在 docker-compose.yml 里启用这个服务

---

## 与 ARCHITECTURE.md 的关联

- 架构图层：「数据采集层」V2 占位
- Broker 抽象层：trading_agent 已为 stock 接入预留
- ARCHITECTURE.md v1.9：股票模块完整方案（QMT 接入决策）

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版占位 |
