// trading_agent/adapters/llm_client — HTTP 客户端 → llm_scorer /emergency_analysis
package llm_client

// TODO: 实现 HTTP 客户端调用 llm_scorer 的 /emergency_analysis 端点
// 检测到剧烈波动时触发，30s timeout
// 5 分钟防抖（由上层 domain/risk 控制，不在这里）
