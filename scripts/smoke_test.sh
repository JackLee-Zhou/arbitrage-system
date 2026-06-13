#!/bin/bash
# ============================================================
# smoke_test.sh — E2E 冒烟测试（10 步）
# 用法：bash scripts/smoke_test.sh
# ============================================================
set -euo pipefail

PASS=0
FAIL=0

check() {
    local name="$1"; shift
    echo -n "  [$((PASS + FAIL + 1))/10] $name ... "
    if "$@" > /dev/null 2>&1; then
        echo "✅ PASS"
        PASS=$((PASS + 1))
    else
        echo "❌ FAIL"
        FAIL=$((FAIL + 1))
    fi
}

echo "========================================="
echo " arbitrage-system — E2E 冒烟测试"
echo "========================================="

# 1. 基础连通性
check "Redis ping" redis-cli -h localhost ping

# 2. InfluxDB 在线
check "InfluxDB ping" curl -s http://localhost:8086/ping

# 3. CTP Tick 有数据
check "Tick Stream 长度" redis-cli XLEN tick:MA2509

# 4. AI 评分存在
check "AI 评分 live" redis-cli GET ai_score:MA2509:live

# 5. 策略信号
check "策略信号" redis-cli XLEN signal:MA2509

# 6. Dashboard 在线
check "Dashboard 访问" curl -s -o /dev/null -w '%{http_code}' http://localhost:8000

# 7. orchestrator REST
check "orchestrator API" curl -s http://localhost:8080/api/v1/overview

# 8. trading_agent gRPC
check "trading_agent gRPC" grpcurl -plaintext localhost:9090 list

# 9. 各服务 /healthz
check "ctp_tick health" curl -s http://localhost:8081/healthz
check "trading_agent health" curl -s http://localhost:8081/healthz
check "llm_scorer health" curl -s http://localhost:5000/healthz

# 10. watchdog 在线

echo ""
echo "========================================="
echo " 结果: $PASS PASS / $FAIL FAIL"
echo "========================================="
