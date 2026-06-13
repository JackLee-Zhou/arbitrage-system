#!/bin/bash
# ============================================================
# deploy.sh — 部署到阿里云 ECS
# 用法：ECS_HOST=your-ip ECS_USER=ecs-user bash scripts/deploy.sh
# ============================================================
set -euo pipefail

ECS_HOST="${ECS_HOST:?请设置 ECS_HOST 环境变量}"
ECS_USER="${ECS_USER:-ecs-user}"

echo "🚀 部署到 $ECS_USER@$ECS_HOST ..."

# 同步代码（排除 git、数据卷、密钥）
rsync -avz --delete \
    --exclude '.git' \
    --exclude 'data/' \
    --exclude '.env' \
    --exclude '*.pem' \
    --exclude '*.key' \
    ./ "$ECS_USER@$ECS_HOST:~/arbitrage-system/"

# 远程重启
ssh "$ECS_USER@$ECS_HOST" << 'ENDSSH'
    cd ~/arbitrage-system
    docker compose --profile full up -d --build
    docker compose ps
ENDSSH

echo "✅ 部署完成"
