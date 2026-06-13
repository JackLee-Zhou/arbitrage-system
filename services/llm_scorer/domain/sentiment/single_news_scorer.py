"""新闻情感分析器 — 订阅 news_stream，对每条新闻打情感分"""
# TODO: 实现新闻情感打分
# 订阅 Redis Stream news_stream
# 每条新闻调 LLM 打情感分（-1 ~ +1）
# 提取关键事件类型（库存/价格/政策/事故/产能/物流）
