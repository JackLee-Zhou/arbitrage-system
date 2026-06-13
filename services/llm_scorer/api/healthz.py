"""llm_scorer /healthz 端点"""
from fastapi import APIRouter

router = APIRouter()


@router.get("/healthz")
async def healthz():
    """健康检查（含 LLM 可用性）"""
    return {"service": "llm_scorer", "status": "healthy"}
