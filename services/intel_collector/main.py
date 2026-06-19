"""intel_collector — 舆情统一采集服务入口"""
import asyncio
import logging
import os

from fastapi import FastAPI

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(name)s] %(levelname)s: %(message)s",
)
logger = logging.getLogger("intel_collector")

app = FastAPI(title="intel_collector", version="0.1.0")


@app.get("/healthz")
async def healthz():
    return {"service": "intel_collector", "status": "healthy"}


@app.get("/metrics")
async def metrics():
    return {"service": "intel_collector", "metrics": {}}


async def fetch_loop():
    """多源并发抓取主循环"""
    logger.info("Intel fetch loop started")
    while True:
        try:
            # TODO: 加载配置 → 按源类型并发抓取 → 解析 → 清洗 → 去重 → XADD
            logger.info("Fetch tick — data source not available")
        except Exception as e:
            logger.error(f"Fetch error: {e}")
        await asyncio.sleep(300)  # 默认 5 分钟


async def main():
    # 启动抓取循环
    asyncio.create_task(fetch_loop())

    # 启动 FastAPI
    import uvicorn
    port = int(os.getenv("HTTP_PORT", "5001"))
    config = uvicorn.Config(app, host="0.0.0.0", port=port, log_level="info")
    server = uvicorn.Server(config)
    await server.serve()


if __name__ == "__main__":
    asyncio.run(main())
