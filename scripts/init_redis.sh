#!/bin/bash
# ============================================================
# init_redis.sh — 初始化 Redis Streams + KV 键
# ============================================================
set -euo pipefail

SYMBOL="${1:-MA2509}"

redis-cli <<EOF
# 创建 Streams（如果不存在）
XGROUP CREATE tick:${SYMBOL} trading_agent_group \$ MKSTREAM
XGROUP CREATE signal:${SYMBOL} dashboard_group \$ MKSTREAM
XGROUP CREATE order_event orchestrator_group \$ MKSTREAM

# 写入初始状态
SET ai_score:${SYMBOL}:live '{"score":0,"confidence":0,"timestamp":0}'
SET ai_score:${SYMBOL}:shadow '{"score":0,"confidence":0,"timestamp":0}'
SET indicator:${SYMBOL}:current '{"values":[]}'
SET engine_status '{"mode":"normal","started_at":0}'
SET position:current '{"positions":[]}'

echo "✅ Redis 初始化完成: symbol=${SYMBOL}"
EOF
