"""llm_scorer /metrics 端点"""
from fastapi import APIRouter

router = APIRouter()


@router.get("/metrics")
async def metrics():
    """Prometheus 指标端点"""
    return {"service": "llm_scorer", "metrics": {}}
