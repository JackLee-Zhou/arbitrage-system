// orchestrator/adapters/trading_client — gRPC 流式订阅
package trading_client

// TODO: 实现 gRPC 流式订阅 trading_agent.SubscribeStateChanges
// 接收流式推送：持仓变化、告警事件、引擎状态变更
// 转发到 WebSocket push_broker 推送给 dashboard
