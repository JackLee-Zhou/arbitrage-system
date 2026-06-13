"""RSS 源注册表 — 动态管理（M8 阶段）

存储：Redis KV rss_sources:<id> + InfluxDB 历史
CRUD 操作：新增/查看/编辑/删除 RSS 源
⚠️ 只有非交易时段可以新增/修改源配置（走交易时段守卫）
"""
# TODO: M8 实现动态 RSS 源管理
