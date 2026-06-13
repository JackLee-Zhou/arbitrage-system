"""LLM 网关 — 二级熔断（DeepSeek → GLM-4 → 停机）

熔断策略：
1. 主：DeepSeek-V3（OpenAI 兼容 API）
2. 备：智谱 GLM-4
3. 双挂：停机 + 钉钉告警 + 写 Redis "AI_DEAD" 标志
"""
# TODO: 实现二级熔断网关
# - 超时 60s（常规）/ 30s（紧急）
# - 失败自动切换
# - 双挂时更新 Redis engine_status 为 "ai_dead"
