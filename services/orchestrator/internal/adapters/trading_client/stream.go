// orchestrator/adapters/trading_client — gRPC 流式订阅（骨架）
// trading_agent 实现 SubscribeStateChanges 后，orchestrator 通过此文件接入实时推送
package trading_client

import (
	"context"
	"log"
	"sync"
)

// StateChangeHandler 状态变化回调（由调用方注入，收到推送后执行）
type StateChangeHandler func(event StateChangeEvent)

// StateChangeEvent trading_agent 推送的状态变化事件
type StateChangeEvent struct {
	Type    string                 `json:"type"`    // "position_change" / "order_update" / "alert" / "engine_status"
	Channel string                 `json:"channel"` // WebSocket channel 名
	Data    map[string]interface{} `json:"data"`
}

// StreamSubscription gRPC 流式订阅管理器（预留接口）
// 等 trading_agent 的 SubscribeStateChanges RPC 可用后：
//  1. 在 Client 上新增 SubscribeStateChanges(ctx, handler) 方法
//  2. 内部调用 c.conn.NewStream(...) 建立双向流
//  3. 后台 goroutine 循环 Recv()，收到消息调 handler(event)
//  4. 连接断开自动重连（指数退避）
type StreamSubscription struct {
	target   string
	handler  StateChangeHandler
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

// SubscribeStateChanges 订阅 trading_agent 的状态变化流
// 当前为骨架实现：预留接口，等 proto 生成后接入真实 gRPC 流
// handler 在后台 goroutine 中调用，需自行处理并发安全
func (c *Client) SubscribeStateChanges(ctx context.Context, handler StateChangeHandler) (*StreamSubscription, error) {
	sub := &StreamSubscription{
		target:  c.target,
		handler: handler,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	// TODO: 等 trading_agent 实现 SubscribeStateChanges RPC 后：
	// stream, err := c.conn.NewStream(ctx, ...)
	// go sub.receiveLoop(stream)

	log.Printf("[trading_client] SubscribeStateChanges: gRPC stream not yet available (trading_agent not implemented), returning skeleton subscription")
	close(sub.doneCh) // 立即标记为完成（骨架）

	return sub, nil
}

// Stop 停止流订阅（安全重复调用）
func (s *StreamSubscription) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		<-s.doneCh
		log.Printf("[trading_client] stream subscription stopped: target=%s", s.target)
	})
}

// receiveLoop 后台接收循环（等 proto 生成后实现）
// func (s *StreamSubscription) receiveLoop(stream grpc.ClientStream) {
//     defer close(s.doneCh)
//     for {
//         select {
//         case <-s.stopCh:
//             return
//         default:
//             event := StateChangeEvent{}
//             if err := stream.RecvMsg(&event); err != nil {
//                 if err != io.EOF {
//                     log.Printf("[trading_client] stream recv error: %v", err)
//                 }
//                 return
//             }
//             if s.handler != nil {
//                 s.handler(event)
//             }
//         }
//     }
// }
