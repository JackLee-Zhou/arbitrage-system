# scripts/ — 运维脚本集合

> 不属于任何微服务的运维工具。所有脚本都是**可重入的**（多次运行结果一致）。

---

## 脚本清单

| 脚本 | 触发时机 | 执行环境 | 风险等级 |
|---|---|---|---|
| `init_server.sh` | 首次登录新 ECS（M1 阶段） | 阿里云 ECS（root sudo） | ⚠️ 中（改系统配置） |
| `init_redis.sh` | docker compose 第一次启动后 | 任意（连得上 Redis 即可） | 🟢 低（仅初始化 Key） |
| `seed_data.sh` | 测试 / 演示用 | 任意 | 🟢 低（写测试数据） |
| `smoke_test.sh` | 每次部署后 / 每次重大变更后 | 阿里云 ECS（生产环境） | ⚠️ 中（会产生模拟订单） |
| `deploy.sh` | 部署新版本到生产 | Mac 本地（推送镜像 + ssh） | 🔴 高（生产部署） |

---

## init_server.sh — 服务器首次加固

### 用途
阿里云 ECS 首次登录后**第一件事**就是跑这个脚本。完成 11 步安全加固。

### 执行环境
- 阿里云 ECS（Ubuntu 22.04）
- 用 `ecs-user` 登录，脚本内部走 sudo

### 执行步骤
```bash
# 1. 用 ecs-user 通过密钥登录
ssh -i ~/.ssh/aliyun_trader.pem ecs-user@<ECS-公网IP>

# 2. clone 仓库
git clone <repo-url> ~/arbitrage-system
cd ~/arbitrage-system

# 3. 跑加固脚本
bash scripts/init_server.sh
```

### 11 步加固内容

| # | 动作 | 目的 |
|---|---|---|
| 1 | apt update + 装基础工具（curl/git/vim/htop/ufw/fail2ban） | 基础环境 |
| 2 | `usermod -aG docker ecs-user` | 免 sudo 跑 docker |
| 3 | 修改 SSH 端口（22 → 22922） | 减少扫描噪音 |
| 4 | `PasswordAuthentication no` + `PermitRootLogin no` | 关密码登录 |
| 5 | 配置 fail2ban（SSH 失败 3 次封 1 小时） | 防暴力破解 |
| 6 | ufw 防火墙（默认拒绝入方向 + 仅放行 自定义 SSH + Tailscale 子网） | 主机层防火墙 |
| 7 | 安装 Docker CE + Docker Compose v2 | 容器运行时 |
| 8 | 安装 Tailscale + 接入 tailnet | 内网通信 |
| 9 | 时区 Asia/Shanghai + NTP 同步 | 交易时间必须准 |
| 10 | swap 4G + 内核参数调优 | 防内存波动 |
| 11 | MOTD 横幅（防误登录其他机器） | 视觉提醒 |

### 验证
```bash
# 重新登录后
ssh -p 22922 -i ~/.ssh/aliyun_trader.pem ecs-user@<ECS-公网IP>

# 检查 Docker
docker ps

# 检查 Tailscale
tailscale status
```

### 风险点
- ⚠️ SSH 端口改完之后**老的 22 端口连不上**，要用新端口
- ⚠️ 跑脚本前确认 ssh 密钥已配置好，否则可能锁死
- ⚠️ 阿里云安全组也要开 22922 端口（脚本不会自动改云防火墙）

---

## init_redis.sh — Redis Key 初始化

### 用途
docker compose 第一次起来后，往 Redis 写一些必备的初始数据：

- 默认策略配置（shadow 模式）
- 默认风控参数
- 默认交易日历

### 执行
```bash
bash scripts/init_redis.sh
```

### 内容示例
```bash
redis-cli -h ali-trade SET strategies:default '{"name":"ma_cross_v1","mode":"shadow",...}'
redis-cli -h ali-trade SET risk:limits '{"max_position":2,"daily_loss_pct":0.02}'
redis-cli -h ali-trade SADD strategies:active "ma_cross_v1"
```

### 幂等性
- 已存在的 Key 不覆盖（除非加 `--force`）
- 适合多次跑

---

## seed_data.sh — 测试数据写入

### 用途
开发 / 演示时用，往 InfluxDB 写一些假 Tick + 假 AI 评分，用于 Dashboard 演示效果。

### 执行
```bash
bash scripts/seed_data.sh --hours 24
# 写入最近 24 小时的模拟数据
```

### 注意
- ⚠️ **不要在生产实盘环境跑**（会污染真实数据）
- 脚本会检查 `INFLUXD_BUCKET` 是否含 `_test` 后缀，否则拒绝执行

---

## smoke_test.sh — E2E 冒烟测试

### 用途
每次部署 / 重大改动后，跑一遍 10 步端到端测试，验证全链路通畅。

### 执行
```bash
bash scripts/smoke_test.sh
```

### 10 步测试清单

| # | 测试项 | 通过条件 |
|---|---|---|
| 1 | 基础连通性 | `redis-cli ping` + `curl :8086/ping` 全 OK |
| 2 | CTP 行情接入 | `XLEN tick:MA2509` > 0 |
| 3 | AI 评分写入 | `redis-cli get ai_score:MA2509:live` 返回完整 JSON |
| 4 | 策略信号生成 | trading_agent 日志含 `SIGNAL: BUY/SELL/HOLD` |
| 5 | 模拟报单 | trading_agent 日志含 `OnRtnOrder` + `OnRtnTrade` |
| 6 | 风控拦截 | 极端参数下日志含 `RISK_BLOCK` |
| 7 | Dashboard 展示 | `curl http://orchestrator:8080/api/v1/overview` 200 |
| 8 | 一键平仓 | `POST /api/v1/emergency_close` 后 SimNow 收到平仓 |
| 9 | AI 评分老化 | 停 llm_scorer 31min → trading_agent 切 "AI=0 模式" + 推钉钉 |
| 10 | 断线重连 | `docker kill trading_agent` → 1 秒内自动重启 + 状态恢复 |

### 退出码
- `0`：全部通过
- `非 0`：第几步失败

### 风险
- ⚠️ 第 5 步会真的提交模拟订单（仅 SimNow 环境无害）
- ⚠️ 第 8 步会真的平仓（实盘环境慎用，先确认是 SimNow）

---

## deploy.sh — 一键部署

### 用途
本地 Mac 上一键完成：
1. 构建所有服务镜像
2. 推送到镜像仓库（或直接 ssh + scp）
3. 远程拉取新镜像
4. 滚动重启
5. 跑冒烟测试

### 执行
```bash
# 标准部署
bash scripts/deploy.sh

# 仅部署一个服务
bash scripts/deploy.sh trading_agent

# 跳过冒烟测试（不推荐）
bash scripts/deploy.sh --skip-smoke
```

### 部署流程
```
本地构建镜像
  ↓
推送到 Docker Hub / 阿里云 ACR
  ↓
SSH 到 ECS
  ↓
docker compose pull
  ↓
docker compose up -d --no-deps <service>
  ↓
跑 smoke_test.sh
  ↓
失败则自动回滚到上一版本
```

### 风险点
- 🔴 这是**生产部署**，每次执行前确认：
  1. 是否在交易时段？（建议非交易时段部署）
  2. 当前是否有持仓？（部署前先全平仓）
  3. SimNow 还是实盘？（实盘必须额外确认）
- ⚠️ deploy.sh 会先跑 smoke_test，失败自动回滚

### 版本标签
镜像 tag = git commit short hash（如 `trading_agent:a1b2c3d`）
便于回滚到任意历史版本：
```bash
docker compose pull trading_agent:a1b2c3d  # 拉指定版本
```

---

## 通用约定

### 所有脚本必须满足

1. **可重入**：跑 N 次效果与跑 1 次相同
2. **set -euo pipefail**：错误立即退出，未定义变量报错
3. **日志清晰**：每步用 `echo "🔧 Step X: ..."` 标识进度
4. **失败回滚**：能回滚的尽量回滚，不能回滚的明确报错
5. **支持 --dry-run**：先看会做什么再决定要不要做（高风险脚本必备）

### 脚本头模板

```bash
#!/usr/bin/env bash
set -euo pipefail

# Script: init_server.sh
# Purpose: 阿里云 ECS 首次登录加固
# Usage: bash scripts/init_server.sh [--dry-run]
# Risk: ⚠️  Medium - 修改系统配置（SSH 端口、防火墙等）

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly DRY_RUN="${1:-}"

log() {
    echo "[$(date +'%H:%M:%S')] $*"
}

run() {
    if [[ "$DRY_RUN" == "--dry-run" ]]; then
        log "DRY_RUN: $*"
    else
        eval "$@"
    fi
}

# 业务逻辑
log "🔧 Step 1: 更新系统..."
run "sudo apt update && sudo apt upgrade -y"
# ...
```

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版，5 个核心脚本规范 |
