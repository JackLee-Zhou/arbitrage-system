// orchestrator/api/websocket — 消息分发器
package websocket

// TODO: 实现消息分发器
// 接收来自 Redis 订阅 / gRPC stream 的消息
// 按 channel 分发给已订阅的 WebSocket 客户端
// 支持 snapshot / delta / event 三种消息类型
