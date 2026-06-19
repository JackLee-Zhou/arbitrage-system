"""配置管理 — 从环境变量加载"""
import os


class Config:
    """LLM Scorer 配置"""

    def __init__(self):
        self.deepseek_api_key = os.getenv("DEEPSEEK_API_KEY", "")
        self.deepseek_base_url = os.getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com")
        self.deepseek_model = os.getenv("DEEPSEEK_MODEL", "deepseek-chat")

        self.glm_api_key = os.getenv("GLM_API_KEY", "")
        self.glm_base_url = os.getenv("GLM_BASE_URL", "https://open.bigmodel.cn/api/paas/v4")
        self.glm_model = os.getenv("GLM_MODEL", "glm-4")

        self.redis_addr = os.getenv("REDIS_ADDR", "redis:6379")
        self.symbol = os.getenv("SYMBOL", "")
        self.score_interval_sec = int(os.getenv("SCORE_INTERVAL_SEC", "300"))
        self.score_ttl_sec = int(os.getenv("SCORE_TTL_SEC", "1800"))
        self.http_port = int(os.getenv("HTTP_PORT", "5000"))
