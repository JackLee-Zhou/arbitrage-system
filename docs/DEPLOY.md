# 部署手册

## 前置条件

- 阿里云 ECS 4C8G u1 实例（Ubuntu 22.04）
- 已执行 `scripts/init_server.sh` 安全加固
- SimNow 模拟账户已申请
- Docker + Docker Compose v2 已安装

## 部署步骤

### 1. 克隆仓库

```bash
git clone <repo-url> ~/arbitrage-system
cd ~/arbitrage-system
```

### 2. 配置环境变量

```bash
# 复制模板
cp .env.example .env

# 编辑 .env，填写以下必填项：
#   INFLUXDB_ADMIN_USER     — InfluxDB 管理员用户名
#   INFLUXDB_ADMIN_PASSWORD — InfluxDB 管理员密码
#   INFLUXDB_TOKEN          — InfluxDB API Token
#   ECS_HOST                — ECS 公网 IP 或域名
#   DINGTALK_WEBHOOK_URL    — 钉钉群机器人 Webhook（可选）

# 各服务 .env（按需修改）
cp services/ctp_tick/.env.example services/ctp_tick/.env
cp services/db_writer/.env.example services/db_writer/.env
cp services/watchdog/.env.example services/watchdog/.env
# ... 其他服务同理
```

### 3. 服务注册（自动发现）

每个服务启动时通过 `pkg/registry` 向 Redis 注册自己的三元组（host:port + 名称 + 能力）。watchdog 从 `services:*` 自动发现所有服务，无需硬编码地址。

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `SERVICE_HOST`（ctp_tick）| 空→容器名 | 本机可达地址，本地设 `localhost`，Docker 留空自动用容器名 |
| `DB_WRITER_SERVICE_HOST` | 空→容器名 | 同上 |
| `REDIS_ADDR`（watchdog）| `redis:6379` | watchdog 连接 Redis 获取服务列表 |

```bash
# 本地开发：显式设 localhost
SERVICE_HOST=localhost REDIS_ADDR=localhost:6379 ./ctp_tick
DB_WRITER_SERVICE_HOST=localhost DB_WRITER_REDIS_ADDR=localhost:6379 ./db_writer
REDIS_ADDR=localhost:6379 ./watchdog

# Docker Compose：留空，自动用容器名（如 ctp_tick:8081）
```

### 4. 启动服务

```bash
# 完整启动（所有 9 个服务）
docker compose --profile full up -d --build

# 或按需启动
docker compose --profile futures up -d --build   # 期货侧核心
docker compose --profile ai-only up -d --build   # AI 评分侧
docker compose --profile ui up -d --build        # 前端侧
```

### 5. 验证健康状态

```bash
# 检查所有容器运行状态
docker compose ps

# 挨个检查健康端点
curl -s http://localhost:8081/healthz   # ctp_tick / trading_agent
curl -s http://localhost:5000/healthz   # llm_scorer
curl -s http://localhost:8080/healthz   # orchestrator

# 冒烟测试
bash scripts/smoke_test.sh
```

## 日常运维

### 查看日志

线上服务日志全部输出到容器 stdout/stderr，通过 Docker 查看：

```bash
# 查看单个服务日志（最近 100 行）
docker compose logs --tail=100 ctp_tick

# 实时跟踪
docker compose logs -f db_writer

# 跟踪 + 最近 50 行
docker compose logs -f --tail=50 watchdog

# 同时看多个服务
docker compose logs -f ctp_tick db_writer watchdog

# 全部服务（逐服务滚动）
docker compose logs -f

# 加时间戳
docker compose logs -f --timestamps trading_agent
```

### 重启单个服务

```bash
docker compose up -d --no-deps --build db_writer
```

### 进入容器调试

```bash
docker compose exec ctp_tick sh
docker compose exec db_writer sh
```

### 查看资源占用

```bash
docker stats
```

### 更新部署

```bash
# 方式一：本地 make 命令
make deploy

# 方式二：手动
rsync -avz --exclude '.git' --exclude 'data/' --exclude '.env' \
  ~/arbitrage-system/ $ECS_USER@$ECS_HOST:~/arbitrage-system/
ssh $ECS_USER@$ECS_HOST "cd ~/arbitrage-system && docker compose --profile full up -d --build"
```

## 各服务端口

| 服务 | 端口 | 用途 |
|------|------|------|
| redis | 6379 | Stream + KV |
| influxdb | 8086 | 时序存储 /health + API |
| ctp_tick | 8081 | /healthz + /metrics |
| llm_scorer | 5000 | /healthz |
| trading_agent | 8081 | /healthz（同端口） |
| trading_agent | 9090 | gRPC |
| orchestrator | 8080 | REST + WebSocket |
| dashboard_web | 8000 | React 前端 |
| db_writer | 8081 | /healthz + /metrics（同端口） |
| watchdog | — | 无暴露端口 |
