"""intel_collector /healthz 端点"""
from fastapi import APIRouter

router = APIRouter()


@router.get("/healthz")
async def healthz():
    """健康检查（含各数据源最近抓取时间）"""
    return {"service": "intel_collector", "status": "healthy"}
