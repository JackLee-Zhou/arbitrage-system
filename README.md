# arbitrage-system

> AI 辅助日内趋势交易系统 — 主交易甲醇期货 MA 主力合约

## 项目简介

云端 Go 交易引擎 + Python AI Agent 的全自动化日内趋势交易系统：
- **Go 交易核心**：CTP 行情接入、技术指标、策略执行、报单、风控
- **Python AI Agent**：DeepSeek/GLM 多 Agent 基本面分析，输出置信度评分
- **全部部署**：阿里云上海 4C8G ECS + Docker Compose

## 状态

📋 **规划中** — 完整方案见 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)

## 文档

- [架构与实施方案](docs/ARCHITECTURE.md) — 完整设计文档
- [架构红线](ARCHITECTURE_GUARDRAILS.md) — 正式数据流、禁止业务 mock/stub、禁止写死关注标的
- [安全加固方案](SECURITY_HARDENING.md) — 全链路加密、mTLS、指令签名、密钥管理
- 跨语言契约（待写）
- 部署手册（待写）
- 排障指南（待写）

## License

私有项目，未开源。
