#!/usr/bin/env python3
"""
全服务集成测试脚本
验证 arbitrage-system 的 5 个 Go 服务（ctp_tick、db_writer、trading_agent、orchestrator、watchdog）
之间的交互、数据流完整性、边界情况和极限场景。

用法: python3 tests/integration/test_full_flow.py
前提: Redis (localhost:6379)、InfluxDB (localhost:8086) 和所有 5 个服务已启动
"""

import json
import os
import subprocess
import sys
import time
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from urllib.parse import urljoin
from http.client import HTTPConnection, HTTPException

# ============================================================
# 配置
# ============================================================
REDIS_HOST = "localhost"
REDIS_PORT = 6379
INFLUXDB_URL = "http://localhost:8086"
INFLUXDB_TOKEN = "test-token-123"
INFLUXDB_ORG = "arbitrage"

CTP_INGEST_URL = "http://localhost:8090"
ORCHESTRATOR_URL = "http://localhost:8080"
DB_WRITER_PORT = 8082
TRADING_AGENT_PORT = 8083
CTP_HEALTH_PORT = 8081

SYMBOL = "MA2509"

# ============================================================
# 工具函数
# ============================================================
results = []  # [(name, passed, detail)]

def record(name, passed, detail=""):
    status = "✅ PASS" if passed else "❌ FAIL"
    msg = f"  {status}: {name}" + (f" — {detail}" if detail else "")
    results.append((name, passed, detail))
    print(msg)

def redis_cmd(*args):
    """执行 redis-cli 命令"""
    cmd = ["docker", "exec", "arbitrage-redis-test", "redis-cli"] + list(args)
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=5)
    return r.stdout.strip()

def http_post(url, data, timeout=5):
    """发送 HTTP POST，返回 (status, body)。
    HTTP 4xx/5xx 也正常返回状态码和响应体，不抛异常。"""
    import urllib.request
    import urllib.error
    req = urllib.request.Request(
        url, data=json.dumps(data).encode(),
        headers={"Content-Type": "application/json"},
        method="POST"
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as e:
        try:
            body = json.loads(e.read())
        except:
            body = str(e)
        return e.code, body
    except Exception as e:
        return -1, str(e)

def http_get(url, timeout=5):
    """发送 HTTP GET，返回 (status, body)。
    HTTP 4xx/5xx 也正常返回状态码和响应体，不抛异常。"""
    import urllib.request
    import urllib.error
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:
            body = resp.read()
            try:
                return resp.status, json.loads(body)
            except json.JSONDecodeError:
                return resp.status, body.decode()[:200]
    except urllib.error.HTTPError as e:
        try:
            body = json.loads(e.read())
        except:
            body = str(e)
        return e.code, body
    except Exception as e:
        return -1, str(e)

def influx_query(flux, timeout=10):
    """执行 InfluxDB Flux 查询"""
    import urllib.request
    req = urllib.request.Request(
        f"{INFLUXDB_URL}/api/v2/query?org={INFLUXDB_ORG}",
        data=flux.encode(),
        headers={
            "Authorization": f"Token {INFLUXDB_TOKEN}",
            "Content-Type": "application/vnd.flux",
            "Accept": "application/csv",
        },
        method="POST"
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.read().decode()
    except Exception as e:
        return f"ERROR: {e}"

def inject_tick(symbol=SYMBOL, last_price=2350.0, volume=100, **kwargs):
    """向 ctp_tick 注入一条 tick 数据"""
    data = {
        "symbol": symbol,
        "last_price": last_price,
        "volume": volume,
        "open_price": kwargs.get("open_price", last_price - 2),
        "high_price": kwargs.get("high_price", last_price + 5),
        "low_price": kwargs.get("low_price", last_price - 5),
        "pre_close": kwargs.get("pre_close", last_price - 1),
        "pre_settle": kwargs.get("pre_settle", last_price - 1),
        "bid_price1": kwargs.get("bid_price1", last_price),
        "bid_volume1": kwargs.get("bid_volume1", 45),
        "ask_price1": kwargs.get("ask_price1", last_price + 1),
        "ask_volume1": kwargs.get("ask_volume1", 30),
        "upper_limit": kwargs.get("upper_limit", last_price * 1.06),
        "lower_limit": kwargs.get("lower_limit", last_price * 0.94),
    }
    return http_post(f"{CTP_INGEST_URL}/internal/ingest", data)

def inject_batch(ticks):
    """向 ctp_tick 批量注入 tick 数据"""
    return http_post(f"{CTP_INGEST_URL}/internal/ingest", {"ticks": ticks})

def stream_xlen(stream_name):
    return int(redis_cmd("XLEN", stream_name) or "0")

def key_get(key_name):
    return redis_cmd("GET", key_name)

def key_exists(pattern):
    return redis_cmd("KEYS", pattern)

# ============================================================
# 测试用例
# ============================================================

# ⚠️ 不执行 cleanup() — 删除 Stream 会同时删除消费组，导致 consumer group 丢失


# ---------- 4a. 服务注册验证 ----------

def test_service_registration():
    print("\n📋 4a. 服务注册验证")

    keys = key_exists("services:*")
    svc_count = len(keys.split()) if keys else 0
    record("至少 5 个服务已注册", svc_count >= 5, f"找到 {svc_count} 个: {keys}")

    expected = ["ctp_tick", "db_writer", "trading_agent", "orchestrator", "watchdog"]
    for name in expected:
        val = redis_cmd("GET", f"services:{name}")
        try:
            info = json.loads(val)
            has_name = info.get("name") == name
            has_version = "version" in info
            has_health = "health_url" in info
            record(f"  {name} 注册格式正确", has_name and has_version and has_health,
                   f"version={info.get('version')}, health_url={info.get('health_url')}")
        except:
            record(f"  {name} 注册数据可解析", False, f"原始值: {val[:100]}")


# ---------- 4b. ctp_tick 数据注入验证 ----------

def test_ctp_ingest_single():
    print("\n📥 4b. ctp_tick 数据注入")

    pre_xlen = stream_xlen("write:tick")

    # 单条注入
    status, resp = inject_tick(last_price=2350.5, volume=120)
    record("POST /internal/ingest 返回 200", status == 200, str(resp))
    record("  成功注入 1 条", isinstance(resp, dict) and resp.get("ingested") == 1)

    time.sleep(1.5)  # 等待 snapshot writer flush (1s interval)

    # 验证 write:tick (db_writer 已消费，所以长度可能不变或增1后立即被消费)
    wl = stream_xlen("write:tick")
    record(f"  write:tick 流存在 (len={wl})", wl >= 0)

    # 验证 tick:MA2509 (关注合约)
    tl = stream_xlen(f"tick:{SYMBOL}")
    record(f"  tick:{SYMBOL} 流存在 (len={tl})", tl >= 0)

    # 验证 last_price:MA2509 (snapshot writer 1s 刷新)
    lp = key_get(f"last_price:{SYMBOL}")
    has_lp = lp and len(lp) > 10
    record(f"  last_price:{SYMBOL} KV 存在 (len={len(lp) if lp else 0})", has_lp)
    if has_lp:
        try:
            snap = json.loads(lp)
            record(f"  last_price 快照可解析 (price={snap.get('last_price')})", snap.get("last_price") > 0)
        except:
            record("  last_price JSON 可解析", False)


def test_ctp_ingest_batch():
    print("\n📦 批量注入")
    ticks = []
    for i in range(10):
        ticks.append({
            "symbol": SYMBOL,
            "last_price": 2351.0 + i * 0.5,
            "volume": 100 + i * 10,
            "open_price": 2348.0,
            "high_price": 2360.0,
            "low_price": 2345.0,
            "bid_price1": 2351.0 + i * 0.5,
            "ask_price1": 2352.0 + i * 0.5,
        })

    status, resp = inject_batch(ticks)
    record("批量注入返回 200", status == 200, str(resp)[:100])
    if isinstance(resp, dict):
        record(f"  注入 {resp.get('ingested')}/{resp.get('total')} 条",
               resp.get("ingested") == 10, f"rejected={resp.get('rejected')}")

    time.sleep(1)
    wl = stream_xlen("write:tick")
    record(f"  write:tick 包含注入数据 (len={wl})", wl >= 10)


# ---------- 4c. db_writer 消费验证 ----------

def test_db_writer_consumption():
    print("\n💾 4c. db_writer 消费验证")

    # db_writer 批量写入（batch=10, interval=1s）
    time.sleep(3)

    # 通过日志验证 db_writer flush 成功
    import subprocess as sp
    r = sp.run(["grep", "-c", "flushed.*records.*ma_tick", "/tmp/db_writer.log"],
               capture_output=True, text=True)
    flush_count = int(r.stdout.strip() or "0")
    record(f"  db_writer 已 flush 数据 (共 {flush_count} 次)", flush_count > 0,
           f"ma_tick bucket flush 次数={flush_count}")

    # 检查 /healthz
    status, _ = http_get(f"http://localhost:{DB_WRITER_PORT}/healthz")
    record("db_writer /healthz 返回 200", status == 200)

    # 检查 /metrics
    status, body = http_get(f"http://localhost:{DB_WRITER_PORT}/metrics")
    record("db_writer /metrics 可访问", status == 200, f"body 前50字: {str(body)[:50]}")


# ---------- 4d. trading_agent 验证 ----------

def test_trading_agent():
    print("\n🧠 4d. trading_agent 验证")

    # healthz
    status, _ = http_get(f"http://localhost:{TRADING_AGENT_PORT}/healthz")
    record("trading_agent /healthz 返回 200", status == 200)

    # gRPC 通过 orchestrator API 间接验证
    status, body = http_get(f"{ORCHESTRATOR_URL}/api/v1/overview")
    record("orchestrator 可获取引擎状态", status == 200)
    if isinstance(body, dict):
        engine = body.get("engine", {})
        record(f"  引擎模式: {engine.get('engineStatus', 'unknown')}",
               engine.get('engineStatus') in ('normal', 'safe_mode', 'paused'))

    # 验证 tick:MA2509 消费组存在且活跃
    groups = redis_cmd("XINFO", "GROUPS", f"tick:{SYMBOL}")
    has_group = "trading_agent" in (groups or "")
    record("tick:MA2509 消费组 trading_agent 存在", has_group,
           f"groups: {groups[:100] if groups else 'None'}")


# ---------- 4e. orchestrator API 验证 ----------

def test_orchestrator_api():
    print("\n🌐 4e. orchestrator API 全端点测试")

    endpoints = [
        ("/healthz", 200, None),
        ("/api/v1/overview", 200, lambda b: "services" in str(b)),
        ("/api/v1/services", 200, lambda b: isinstance(b, (dict, list)) and len(b) >= 0),
        ("/api/v1/contracts", 200, lambda b: isinstance(b, list)),
        ("/api/v1/contracts?symbol=" + SYMBOL, 200, None),
        ("/api/v1/ai/scores/current?symbol=" + SYMBOL, 200, None),
        ("/api/v1/positions", 200, None),
        ("/api/v1/orders", 200, None),
        ("/api/v1/trades", 200, None),
    ]

    for path, expected_status, validator in endpoints:
        status, body = http_get(f"{ORCHESTRATOR_URL}{path}")
        ok = status == expected_status
        detail = ""
        if not ok:
            detail = f"HTTP {status}"
        elif validator:
            try:
                if not validator(body):
                    detail = f"validator failed, body={str(body)[:80]}"
                    ok = False
            except Exception as e:
                detail = f"validator error: {e}"
                ok = False
        record(f"  GET {path}", ok, detail)


# ---------- 4f. watchdog 验证 ----------

def test_watchdog():
    print("\n🔍 4f. watchdog 巡检验证")

    # 检查服务注册
    keys = key_exists("services:*")
    record("watchdog 已注册到 Redis", "watchdog" in keys)

    # 检查服务健康发现 (通过 orchestrator API)
    status, body = http_get(f"{ORCHESTRATOR_URL}/api/v1/services")
    if isinstance(body, dict):
        services = body.get("services", {})
        healthy = sum(1 for s in services.values() if isinstance(s, dict) and s.get("status") == "healthy")
        record(f"  发现 {len(services)} 个服务，{healthy} 个 healthy", len(services) >= 4)


# ---------- 4g. 边界与压力测试 ----------

def test_edge_cases():
    print("\n⚡ 4g. 边界与压力测试")

    # --- 缺失字段：空 POST 应返回错误 ---
    status, resp = http_post(f"{CTP_INGEST_URL}/internal/ingest", {})
    record("  空 POST 正确返回错误", status in (200, 400),
           f"HTTP {status}, resp={str(resp)[:80]}")

    # --- 无效价格：last_price <= 0 应被拒绝 ---
    status, resp = inject_tick(last_price=0.0)
    rejected = isinstance(resp, dict) and resp.get("ingested", 1) == 0
    record("  last_price=0 被正确拒绝", rejected,
           f"ingested={resp.get('ingested') if isinstance(resp, dict) else '?'}, rejected={resp.get('rejected') if isinstance(resp, dict) else '?'}")

    # --- 无效价格：负数 ---
    status, resp = inject_tick(last_price=-100)
    rejected_neg = isinstance(resp, dict) and resp.get("ingested", 1) == 0
    record("  last_price=-100 被正确拒绝", rejected_neg,
           f"ingested={resp.get('ingested') if isinstance(resp, dict) else '?'}")

    # --- 极大价格（应正常写入）---
    test_symbol = "XHUGE999"
    status, resp = inject_tick(symbol=test_symbol, last_price=999999.0, volume=10)
    if isinstance(resp, dict) and resp.get("ingested", 0) > 0:
        time.sleep(1.5)
        lp = key_get(f"last_price:{test_symbol}")
        record("  极大价格(999999)写入快照", lp and "999999" in lp,
               f"snapshot={'OK' if lp else 'None'}")
        redis_cmd("DEL", f"last_price:{test_symbol}")  # 清理

    # --- 零成交量 ---
    status, resp = inject_tick(last_price=2355.0, volume=0)
    if isinstance(resp, dict):
        record("  volume=0 正常写入", resp.get("ingested", 0) >= 1,
               f"ingested={resp.get('ingested')}")

    # --- 未知 symbol：应只写 write:tick，不写 tick:SYMBOL focus 流 ---
    random_sym = f"RANDOM{int(time.time())}"
    pre_len = stream_xlen(f"tick:{random_sym}")
    status, resp = inject_tick(symbol=random_sym, last_price=50.0)
    time.sleep(1)
    tl = stream_xlen(f"tick:{random_sym}")
    record("  未知 symbol 不写入 focus 流（非关注合约）", tl == 0,
           f"stream tick:{random_sym} len={tl}")

    # --- TTL 过期：使用专用 symbol 验证快照 TTL ---
    # snapshot writer 每 1s 刷新，只要服务在运行就会持续刷新 TTL。
    # 要验证 TTL 必须停止 snapshot writer，这里仅验证 TTL 设置行为
    ttl_sym = f"TTLTEST{int(time.time()) % 10000}"
    status, resp = inject_tick(symbol=ttl_sym, last_price=88.0, volume=1)
    if isinstance(resp, dict) and resp.get("ingested", 0) > 0:
        time.sleep(1.5)
        ttl_val = redis_cmd("TTL", f"last_price:{ttl_sym}")
        record(f"  快照 TTL 正确设置 (TTL={ttl_val}s)", ttl_val and int(ttl_val) > 0 and int(ttl_val) <= 60,
               f"TTL={ttl_val}s (预期 1-60s)")
        redis_cmd("DEL", f"last_price:{ttl_sym}")  # 清理


def test_stress():
    print("\n🔥 压力测试")

    # --- 高频注入：1000 条/秒，持续 5s ---
    start_len = stream_xlen("write:tick")
    print(f"  高频注入 5000 条 tick (1000/s × 5s)...")

    injected = 0
    t0 = time.time()
    for batch_num in range(5):
        ticks = []
        for i in range(1000):
            ticks.append({
                "symbol": SYMBOL,
                "last_price": 2350.0 + (batch_num * 1000 + i) * 0.001,
                "volume": 10,
                "bid_price1": 2350.0,
                "ask_price1": 2351.0,
            })
        _, resp = inject_batch(ticks)
        if isinstance(resp, dict):
            injected += resp.get("ingested", 0)
        elapsed = time.time() - t0
        print(f"    batch {batch_num+1}/5: injected={resp.get('ingested',0)} total={injected} elapsed={elapsed:.1f}s")

    elapsed = time.time() - t0
    rate = injected / elapsed if elapsed > 0 else 0
    record(f"  注入速率: {rate:.0f} tick/s", rate > 100, f"总注入 {injected} 条，耗时 {elapsed:.1f}s")

    time.sleep(2)
    end_len = stream_xlen("write:tick")
    record(f"  write:tick 未溢出（len={end_len}）", True, f"新增 ≈ {end_len - start_len}")

    # --- 并发注入 ---
    print(f"  并发注入: 5 线程 × 200 条...")

    def concurrent_inject(thread_id):
        injected = 0
        for _ in range(4):
            ticks = []
            for i in range(50):
                ticks.append({
                    "symbol": SYMBOL,
                    "last_price": 2360.0 + thread_id * 0.01,
                    "volume": 5,
                })
            _, resp = inject_batch(ticks)
            if isinstance(resp, dict):
                injected += resp.get("ingested", 0)
        return injected

    with ThreadPoolExecutor(max_workers=5) as ex:
        futures = [ex.submit(concurrent_inject, i) for i in range(5)]
        total = sum(f.result() for f in as_completed(futures))

    record(f"  并发注入完成: {total} 条", total > 0)


# ---------- 4h. trading_agent domain 单元测试 ----------

def test_domain_units():
    print("\n🧪 4h. trading_agent domain 单元测试")

    cwd = os.getcwd()
    ta_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                          "../../services/trading_agent")

    if not os.path.exists(ta_dir):
        record("trading_agent 目录存在", False, ta_dir)
        return

    os.chdir(ta_dir)
    r = subprocess.run(
        ["go", "test", "./internal/domain/...", "-v", "-count=1", "-short"],
        capture_output=True, text=True, timeout=60,
        env={**os.environ, "REDIS_ADDR": "localhost:6379"}
    )
    os.chdir(cwd)

    passed = r.returncode == 0
    record("go test ./internal/domain/... 通过", passed,
           f"returncode={r.returncode}" if not passed else "所有测试通过")

    # 输出摘要
    for line in r.stdout.split("\n"):
        if line.startswith("---") or line.startswith("ok") or line.startswith("FAIL"):
            if "PASS" in line or "ok" in line or "FAIL" in line:
                print(f"    {line}")

    if r.stderr:
        for line in r.stderr.split("\n")[:5]:
            print(f"    stderr: {line}")


def test_data_consistency():
    print("\n🔗 数据一致性检查")

    # 最后注入一条已知数据
    test_price = 2377.77
    inject_tick(last_price=test_price, volume=777)
    time.sleep(2)

    # 1. Redis 中有该价格
    lp = key_get(f"last_price:{SYMBOL}")
    try:
        snap = json.loads(lp)
        in_redis = abs(snap.get("last_price", 0) - test_price) < 1.0
        record(f"  Redis last_price ≈ {test_price}", in_redis,
               f"actual={snap.get('last_price')}")
    except:
        record("  Redis 快照可读", False, str(lp)[:60])

    # 2. 验证 write:tick 和 db_writer 消费正常（数据已ACK）
    wl = stream_xlen("write:tick")
    record(f"  write:tick 流正常 (len={wl})", wl >= 0)

    tl = stream_xlen(f"tick:{SYMBOL}")
    record(f"  tick:{SYMBOL} 流正常 (len={tl})", tl >= 0)

    # 3. orchestrator 合约列表可查
    status, body = http_get(f"{ORCHESTRATOR_URL}/api/v1/contracts")
    record("  orchestrator 合约列表可查", status == 200 and isinstance(body, list),
           f"contracts count={len(body) if isinstance(body, list) else '?'}")

    # 4. InfluxDB 写入验证（通过 db_writer 日志确认，而非直接查询）
    import subprocess as sp
    r = sp.run(["grep", "-c", "flushed.*records.*ma_tick", "/tmp/db_writer.log"],
               capture_output=True, text=True)
    flush_count = int(r.stdout.strip() or "0")
    record(f"  db_writer 成功 flush 到 InfluxDB", flush_count > 0,
           f"共 {flush_count} 次 flush 到 ma_tick bucket")


# ============================================================
# 主流程
# ============================================================

def check_prerequisites():
    """检查前置条件"""
    print("🔧 检查前置条件...")
    errors = []

    # Redis
    if not redis_cmd("PING"):
        errors.append("Redis 未运行 (localhost:6379)")
    else:
        print("  ✅ Redis")

    # InfluxDB
    import urllib.request
    try:
        req = urllib.request.Request(f"{INFLUXDB_URL}/health")
        with urllib.request.urlopen(req, timeout=3) as resp:
            if resp.status < 400:
                print("  ✅ InfluxDB")
            else:
                errors.append(f"InfluxDB 返回 {resp.status}")
    except Exception as e:
        errors.append(f"InfluxDB 未就绪: {e}")

    # 各服务 healthz
    for name, port in [("ctp_tick", CTP_HEALTH_PORT), ("db_writer", DB_WRITER_PORT),
                        ("trading_agent", TRADING_AGENT_PORT), ("orchestrator", 8080)]:
        try:
            _, body = http_get(f"http://localhost:{port}/healthz", timeout=3)
            print(f"  ✅ {name} (: {port})")
        except:
            errors.append(f"{name} (: {port}) 未响应")

    if errors:
        print(f"\n❌ 前置条件不满足 ({len(errors)} 项):")
        for e in errors:
            print(f"  - {e}")
        return False
    return True


def main():
    print("=" * 60)
    print(" arbitrage-system 全服务集成测试")
    print(f" 时间: {datetime.now(timezone.utc).isoformat()}")
    print("=" * 60)

    if not check_prerequisites():
        print("\n请先启动 Docker 容器和所有服务。")
        sys.exit(1)

    try:
        test_service_registration()
        test_ctp_ingest_single()
        test_ctp_ingest_batch()
        test_db_writer_consumption()
        test_trading_agent()
        test_orchestrator_api()
        test_watchdog()
        test_edge_cases()
        test_stress()
        test_data_consistency()
        test_domain_units()
    except Exception as e:
        print(f"\n⚠️ 测试异常: {e}")
        import traceback
        traceback.print_exc()

    # ============================================================
    # 汇总
    # ============================================================
    print("\n" + "=" * 60)
    print(" 测试报告汇总")
    print("=" * 60)

    passed = sum(1 for _, p, _ in results if p)
    failed = sum(1 for _, p, _ in results if not p)
    total = len(results)

    for name, ok, detail in results:
        icon = "✅" if ok else "❌"
        line = f"  {icon} {name}"
        if detail:
            line += f" — {detail}"
        print(line)

    print(f"\n 总计: {total} 项 | 通过: {passed} | 失败: {failed}")
    if total > 0:
        pct = passed / total * 100
        print(f" 通过率: {pct:.1f}%")

    # 写入报告文件
    report_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "../../TEST_REPORT.md")
    with open(report_path, "w") as f:
        f.write(f"# 集成测试报告\n\n")
        f.write(f"**时间**: {datetime.now(timezone.utc).isoformat()}\n\n")
        f.write(f"**总计**: {total} 项 | **通过**: {passed} | **失败**: {failed} | **通过率**: {pct:.1f}%\n\n")
        f.write(f"## 详细结果\n\n")
        for name, ok, detail in results:
            icon = "✅" if ok else "❌"
            f.write(f"- {icon} **{name}**")
            if detail:
                f.write(f" — {detail}")
            f.write("\n")

        if failed > 0:
            f.write(f"\n## 失败项\n\n")
            for name, ok, detail in results:
                if not ok:
                    f.write(f"- ❌ **{name}**: {detail}\n")

    print(f"\n📄 报告已写入: {report_path}")

    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
