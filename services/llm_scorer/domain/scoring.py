"""评分调度器 — 协调多 Agent 评分流程"""
# TODO: 实现评分流程编排
# 1. 从 Redis 获取最新 Tick + 指标
# 2. 并行调用三个子系统 Agent
# 3. Fuser Agent 融合
# 4. 写入 Redis (live + shadow)
