"""llm_scorer POST /emergency_analysis — 紧急分析端点

trading_agent 检测到剧烈波动时调用此端点。
30s timeout，生成紧急评分写入 Redis ai_score:MA2509:emergency (TTL 10min)
"""
from fastapi import APIRouter

router = APIRouter()


@router.post("/emergency_analysis")
async def emergency_analysis(symbol: str = ""):
    """紧急分析端点（trading_agent 触发）

    接收波动数据 → 抓最近 30 分钟新闻 → 专用 EMERGENCY_PROMPT 调 LLM
    → 写 ai_score:{symbol}:emergency（TTL 10min）→ 推钉钉
    """
    # TODO: 实现完整紧急分析流程
    return {
        "symbol": symbol,
        "score": None,
        "suggested_action": "pause_new_orders",
        "status": "pending_implementation",
    }
