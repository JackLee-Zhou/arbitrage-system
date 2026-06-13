"""Redis 写入器 — 写新闻事件到 Redis Streams

写入流：
- news_stream：新闻事件流（llm_scorer 订阅做情感打分）
- write:sentiment：持久化流（db_writer 消费写入 InfluxDB）
"""
# TODO: 实现 Redis Streams 写入（XADD）
