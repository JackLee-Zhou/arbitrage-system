// orchestrator/api/websocket — WebSocket 推送协议定义
package websocket

// WSMessage WebSocket 消息格式
type WSMessage struct {
	Type    string      `json:"type"`    // "snapshot" / "delta" / "event"
	Channel string      `json:"channel"` // "system_status" / "positions" / "scores" / "alerts"
	TS      int64       `json:"ts"`      // unix ms
	Data    interface{} `json:"data"`
}

// Channels 支持的推送 channel
var Channels = []string{
	"system_status",
	"positions",
	"scores",
	"alerts",
	"orders",
	"trades",
	"sentiment",
}
