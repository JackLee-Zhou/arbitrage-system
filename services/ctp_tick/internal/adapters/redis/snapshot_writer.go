// ctp_tick/adapters/redis — 全市场最新价快照写入 Redis
package redis

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

// ============================================================
// 常量（禁止魔法数字 — ENGINEERING_GUIDE.md 规则 6）
// ============================================================

const (
	// SnapshotWriteInterval 快照批量写入 Redis 的间隔
	SnapshotWriteInterval = 1 * time.Second

	// SnapshotTTLSeconds Redis KV 快照过期时间（秒）
	// 断线后自动老化，确保 stale 数据不会残留在 Redis 中
	SnapshotTTLSeconds int64 = 60
)

// ============================================================
// SnapshotWriter — 后台定期批量写入 Redis
// ============================================================

// SnapshotWriter 读取 LastPriceStore 中的全市场快照，每 1 秒批量写入 Redis。
// 写入方式：SET last_price:<symbol> <JSON> EX 60
// 通过节流到 1s 一次减少 Redis 写入压力。
type SnapshotWriter struct {
	store  *domain.LastPriceStore
	client pkgredis.RedisClient
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSnapshotWriter 创建快照写入器并启动后台 goroutine。
func NewSnapshotWriter(store *domain.LastPriceStore, client pkgredis.RedisClient) *SnapshotWriter {
	w := &SnapshotWriter{
		store:  store,
		client: client,
		stopCh: make(chan struct{}),
	}
	w.wg.Add(1)
	go w.loop()
	return w
}

// loop 后台 goroutine，每 SnapshotWriteInterval 批量写入一次。
func (w *SnapshotWriter) loop() {
	defer w.wg.Done()
	log.Printf("[snapshot_writer] started, interval=%v, ttl=%ds", SnapshotWriteInterval, SnapshotTTLSeconds)

	ticker := time.NewTicker(SnapshotWriteInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			log.Printf("[snapshot_writer] stopped")
			return
		case <-ticker.C:
			w.flush()
		}
	}
}

// flush 将当前所有快照批量写入 Redis。
func (w *SnapshotWriter) flush() {
	snapshots := w.store.Snapshot()
	if len(snapshots) == 0 {
		return
	}

	ctx := context.Background()
	written := 0
	for symbol, snap := range snapshots {
		data, err := json.Marshal(snap)
		if err != nil {
			log.Printf("[snapshot_writer] json marshal failed for %s: %v", symbol, err)
			continue
		}
		key := "last_price:" + symbol
		if err := w.client.Set(ctx, key, string(data), SnapshotTTLSeconds); err != nil {
			log.Printf("[snapshot_writer] SET %s failed: %v", key, err)
			continue
		}
		written++
	}
	if written > 0 {
		log.Printf("[snapshot_writer] flushed %d snapshots to Redis", written)
	}
}

// Close 优雅关闭：停止写入 goroutine。
func (w *SnapshotWriter) Close() {
	close(w.stopCh)
	w.wg.Wait()

	// 关闭前最后 flush 一次，确保最新数据写入
	w.flush()

	log.Printf("[snapshot_writer] closed")
}
