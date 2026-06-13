"""Prompt 模板 — 各 Agent 和融合器的 Prompt"""
# 注意：修改 prompt 需要升版本号，便于追溯历史评分

PROMPT_VERSION = "v1.0"

MARKET_PROMPT = """
分析甲醇期货 {symbol} 的市场微观结构：
最近 Tick 数据：{tick_data}
请输出市场结构评分 JSON...
"""

TECHNICAL_PROMPT = """
分析甲醇期货 {symbol} 的技术指标：
指标值：{indicators}
请输出技术评分 JSON...
"""

FUNDAMENTALS_PROMPT = """
分析甲醇期货 {symbol} 的基本面：
库存：{inventory}
基差：{basis}
原油：{oil_price}
请输出基本面评分 JSON...
"""

FUSER_PROMPT = """
综合以下 Agent 分析，给出甲醇期货 {symbol} 的最终日内评分。
市场结构：{market_score}
技术面：{technical_score}
基本面：{fundamentals_score}
请输出融合评分 JSON...
"""

EMERGENCY_PROMPT = """
⚠️ 紧急分析：甲醇期货 {symbol} 出现剧烈波动！
波动详情：{volatility}
最近新闻：{recent_news}
请在 30 秒内给出紧急评分和建议操作 JSON...
"""
