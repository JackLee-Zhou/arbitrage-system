// Package websocket — WebSocket 连接管理 + 实时推送
package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message represents a WebSocket push message
type Message struct {
	Type    string      `json:"type"`
	Channel string      `json:"channel"`
	TS      int64       `json:"ts"`
	Data    interface{} `json:"data"`
}

// Client represents a single WebSocket connection
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	channels map[string]bool
	mu       sync.Mutex
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex

	SnapshotProvider func() map[string]interface{}

	upgrader websocket.Upgrader
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// Run starts the hub event loop (blocks, run in goroutine)
func (h *Hub) Run() {
	deltaThrottle := make(map[string]*Message)
	throttleTicker := time.NewTicker(1 * time.Second)
	defer throttleTicker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

			if h.SnapshotProvider != nil {
				snap := h.SnapshotProvider()
				for channel, data := range snap {
					client.sendMessage(&Message{
						Type:    "snapshot",
						Channel: channel,
						TS:      time.Now().UnixMilli(),
						Data:    data,
					})
				}
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			if msg.Type == "delta" {
				deltaThrottle[msg.Channel] = msg
			} else {
				h.broadcastToAll(msg)
			}

		case <-throttleTicker.C:
			for ch, msg := range deltaThrottle {
				h.broadcastToAll(msg)
				delete(deltaThrottle, ch)
			}
		}
	}
}

func (h *Hub) broadcastToAll(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.isSubscribed(msg.Channel) {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// Broadcast pushes a message to all subscribed clients
func (h *Hub) Broadcast(msg *Message) {
	select {
	case h.broadcast <- msg:
	default:
	}
}

// BroadcastEvent sends an event-type message
func (h *Hub) BroadcastEvent(channel string, data interface{}) {
	h.Broadcast(&Message{Type: "event", Channel: channel, TS: time.Now().UnixMilli(), Data: data})
}

// BroadcastDelta sends a delta-type message (throttled to 1Hz)
func (h *Hub) BroadcastDelta(channel string, data interface{}) {
	h.Broadcast(&Message{Type: "delta", Channel: channel, TS: time.Now().UnixMilli(), Data: data})
}

// HandleUpgrade handles HTTP → WebSocket upgrade
func (h *Hub) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 64),
		channels: make(map[string]bool),
	}
	client.subscribe("system_status")
	client.subscribe("positions")
	client.subscribe("scores")
	client.subscribe("alerts")

	h.register <- client
	go client.writePump()
	go client.readPump()
}

// ClientCount returns connected client count
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ---- Client methods ---------------------------------------------------

func (c *Client) subscribe(channel string) {
	c.mu.Lock()
	c.channels[channel] = true
	c.mu.Unlock()
}

func (c *Client) unsubscribe(channel string) {
	c.mu.Lock()
	delete(c.channels, channel)
	c.mu.Unlock()
}

func (c *Client) isSubscribed(channel string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.channels[channel]
}

func (c *Client) sendMessage(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var cmd struct {
			Type     string   `json:"type"`
			Channels []string `json:"channels"`
		}
		if err := json.Unmarshal(message, &cmd); err != nil {
			continue
		}
		switch cmd.Type {
		case "subscribe":
			for _, ch := range cmd.Channels {
				c.subscribe(ch)
			}
		case "unsubscribe":
			for _, ch := range cmd.Channels {
				c.unsubscribe(ch)
			}
		case "ping":
			_ = c.conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
