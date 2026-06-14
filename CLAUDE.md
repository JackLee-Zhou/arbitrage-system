# CLAUDE.md — AI 协作规范

## 项目概述
`arbitrage-system` 是一套 AI 辅助日内趋势交易系统，主交易甲醇期货 MA 主力合约。云端 Go 交易引擎 + Python AI Agent，全部部署在阿里云上海 ECS Docker Compose。

## 架构原则（不可妥协）
1. **每个服务独立进程、独立 Docker 镜像、独立部署**
2. **服务间只通过 `contracts/` 定义的协议通信**（Protobuf + buf 自动生成代码）
3. **业务逻辑（domain/）零外部依赖，所有 IO 在 adapters/**
4. **每个服务必须有 `/healthz` 和 `/metrics` 端点**
5. **单元测试不能依赖外部服务**（mock/fake 替代）
6. **每个服务启动时必须向 Redis 注册自己的地址、端口、类型**（`pkg/registry`，三元组：`host:port` + name + capabilities），watchdog 和其他服务通过 `services:*` 自动发现，禁止硬编码地址

## 目录结构
```
arbitrage-system/
├── contracts/          # ⭐ SSOT：跨服务通信契约（.proto 文件）
├── services/           # 所有微服务
│   ├── ctp_tick/       # Go — CTP 行情订阅
│   ├── intel_collector/# Python — 舆情采集
│   ├── llm_scorer/     # Python — LLM 评分
│   ├── trading_agent/  # Go — 决策核心（含 indicator）
│   ├── orchestrator/   # Go — 编排层（REST + WebSocket + gRPC 客户端）
│   ├── dashboard_web/  # React — 前端 Dashboard
│   ├── db_writer/      # Go — InfluxDB 串行写入
│   ├── watchdog/       # Go — 健康监控
│   └── stock_tick/     # V2 占位
├── scripts/            # 运维脚本
└── docs/               # 文档
```

## 技术栈
- Go 1.22、Python 3.12、React 17 (CRA)
- Redis 7.2 (Streams + KV)、InfluxDB 2.7
- Protobuf + buf（跨服务契约）
- Docker Compose v2（单机部署）
- gRPC（orchestrator ↔ trading-agent）
- DeepSeek API（主 LLM）+ 智谱 GLM-4（备用）

## 开发规范
- 修改跨服务契约：改 `.proto` → `make gen` → 两端同步
- Go 服务：Hexagonal Architecture（domain/ + adapters/）
- Python 服务：uv 管理依赖，pyproject.toml 配置
- 每个服务独立 go.mod / pyproject.toml
- CI：unit 测试每个 PR 必跑，integration 测试 nightly 跑
