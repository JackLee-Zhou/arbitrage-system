"""代理管理器 — 配置驱动是否走香港代理

国内源：直连（不走代理）
海外源（P3）：走香港跳板 HTTP_PROXY / SOCKS5
社交源（V4）：走香港跳板 + Telegram 国际节点
"""
# TODO: 实现代理路由
# 读取环境变量 HTTP_PROXY/HTTPS_PROXY
# 国内源直连，海外源走代理
