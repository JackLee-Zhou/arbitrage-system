"""Redis 写入器 — 写 AI 评分到 Redis

写入 key：
- ai_score:{symbol}:live — 实际生效评分（TTL 30min）
- ai_score:{symbol}:shadow — 影子评分（仅记录，不参与决策）
- ai_score:{symbol}:emergency — 紧急评分（TTL 10min）
"""
# TODO: 实现 Redis KV 写入 + TTL 设置
