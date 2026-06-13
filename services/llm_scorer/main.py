"""
llm_scorer — LLM 评分服务入口
每 5 分钟触发多 Agent 协作评分，结果写入 Redis
"""
import asyncio
import logging
import os
import signal

from fastapi import FastAPI

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(name)s] %(levelname)s: %(message)s",
)
logger = logging.getLogger("llm_scorer")

_shutdown_requested = False

app = FastAPI(title="llm_scorer", version="0.1.0")


@app.get("/healthz")
async def healthz():
    return {"service": "llm_scorer", "status": "healthy"}


@app.get("/metrics")
async def metrics():
    return {"placeholder": "metrics"}


@app.post("/emergency_analysis")
async def emergency_analysis(symbol: str = "MA2509"):
    """紧急分析端点（trading_agent 触发，30s timeout）"""
    # TODO: 实现紧急评分逻辑
    return {"symbol": symbol, "score": None, "status": "pending"}


async def scoring_loop(symbol: str, interval_sec: int):
    """定时评分主循环"""
    logger.info(f"Scoring loop started: symbol={symbol}, interval={interval_sec}s")
    while True:
        try:
            # TODO: 实现评分流程
            # 1. 获取行情数据（从 Redis）
            # 2. 多 Agent 协作评分
            # 3. 写入 Redis: ai_score:{symbol}:live + :shadow
            logger.info("Score tick — placeholder")
        except Exception as e:
            logger.error(f"Scoring error: {e}")
        await asyncio.sleep(interval_sec)


def _handle_shutdown(signum, frame):
    global _shutdown_requested
    logger.info(f"Received signal {signum}, shutting down...")
    _shutdown_requested = True


async def main():
    signal.signal(signal.SIGINT, _handle_shutdown)
    signal.signal(signal.SIGTERM, _handle_shutdown)

    symbol = os.getenv("SYMBOL", "MA2509")
    interval = int(os.getenv("SCORE_INTERVAL_SEC", "300"))

    # 启动评分循环
    asyncio.create_task(scoring_loop(symbol, interval))

    # 启动 FastAPI
    import uvicorn
    port = int(os.getenv("HTTP_PORT", "5000"))
    config = uvicorn.Config(app, host="0.0.0.0", port=port, log_level="info")
    server = uvicorn.Server(config)
    await server.serve()


if __name__ == "__main__":
    asyncio.run(main())
