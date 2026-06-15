// Package adapters — PriceRelay: Redis last_price snapshot → WebSocket delta push
package adapters

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/arbitrage-system/orchestrator/internal/api/websocket"
	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// PriceSnapshot represents a single contract's latest price data
type PriceSnapshot struct {
	Symbol       string  `json:"symbol"`
	LastPrice    float64 `json:"last_price"`
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	PreClose     float64 `json:"pre_close"`
	ChangePct    float64 `json:"change_pct"`
	Volume       float64 `json:"volume"`
	OpenInterest float64 `json:"open_interest"`
	UpdatedAt    int64   `json:"updated_at"`
}

// PriceRelay polls Redis last_price:* hashes and broadcasts deltas via WebSocket
type PriceRelay struct {
	redis     pkgredis.RedisClient
	hub       *websocket.Hub
	prevCache map[string]float64 // symbol → last_price
	interval  time.Duration
}

// NewPriceRelay creates a new PriceRelay
func NewPriceRelay(redis pkgredis.RedisClient, hub *websocket.Hub) *PriceRelay {
	return &PriceRelay{
		redis:     redis,
		hub:       hub,
		prevCache: make(map[string]float64),
		interval:  1 * time.Second,
	}
}

// Start begins the polling loop (blocks, run in goroutine)
func (r *PriceRelay) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	log.Printf("[price-relay] started, polling every %v", r.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[price-relay] stopped")
			return
		case <-ticker.C:
			r.poll(ctx)
		}
	}
}

func (r *PriceRelay) poll(ctx context.Context) {
	// Discover all last_price:* keys
	keys, err := r.redis.Keys(ctx, "last_price:*")
	if err != nil {
		return
	}

	for _, key := range keys {
		// Extract symbol from key: "last_price:MA2509" → "MA2509"
		symbol := strings.TrimPrefix(key, "last_price:")
		if symbol == "" || symbol == key {
			continue
		}

		// Read hash fields
		fields, err := r.redis.HGetAll(ctx, key)
		if err != nil || len(fields) == 0 {
			continue
		}

		price := parseFloat(fields, "last_price")
		if price == 0 {
			continue
		}

		// Check if price changed since last poll
		prev, existed := r.prevCache[symbol]
		r.prevCache[symbol] = price

		if !existed || price != prev {
			snap := PriceSnapshot{
				Symbol:    symbol,
				LastPrice: price,
				Open:      parseFloat(fields, "open"),
				High:      parseFloat(fields, "high"),
				Low:       parseFloat(fields, "low"),
				PreClose:  parseFloat(fields, "pre_close"),
				ChangePct: parseFloat(fields, "change_pct"),
				Volume:    parseFloat(fields, "volume"),
			}
			if ua, ok := fields["updated_at"]; ok {
				snap.UpdatedAt, _ = strconv.ParseInt(ua, 10, 64)
			}
			if oi, ok := fields["open_interest"]; ok {
				snap.OpenInterest, _ = strconv.ParseFloat(oi, 64)
			}

			// Push delta to all subscribers of contract:<symbol>
			data, _ := json.Marshal(snap)
			var raw map[string]interface{}
			_ = json.Unmarshal(data, &raw)

			channel := "contract:" + symbol
			r.hub.BroadcastDelta(channel, raw)
		}
	}
}

func parseFloat(fields map[string]string, key string) float64 {
	if v, ok := fields[key]; ok {
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}
