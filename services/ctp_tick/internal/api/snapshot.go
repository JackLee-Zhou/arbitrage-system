// ctp_tick/api — /internal/snapshot HTTP 端点
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

// ============================================================
// SnapshotHandler — GET /internal/snapshot
// 提供全市场最新价快照的全量 JSON 查询（供 orchestrator fallback）
// ============================================================

// SnapshotHandler 返回当前内存中全部快照的 JSON。
type SnapshotHandler struct {
	store *domain.LastPriceStore
}

// NewSnapshotHandler 创建快照 HTTP 处理器。
func NewSnapshotHandler(store *domain.LastPriceStore) *SnapshotHandler {
	return &SnapshotHandler{store: store}
}

// ServeHTTP 实现 http.Handler，返回全市场快照 JSON。
//
// 响应格式：
//
//	{
//	  "symbols": {
//	    "MA2509": { "symbol":"MA2509", "last_price":2350.0, ... },
//	    "RB2510": { "symbol":"RB2510", "last_price":3800.0, ... }
//	  },
//	  "count": 2,
//	  "updated_at": "2026-06-14T10:30:00+08:00"
//	}
func (h *SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snapshots := h.store.Snapshot()

	// 计算全局最新更新时间（直接比较 time.Time，避免字符串字典序跨时区错序）
	var latestUpdate time.Time
	var latestUpdateStr string
	for _, s := range snapshots {
		if s.UpdatedAt.After(latestUpdate) {
			latestUpdate = s.UpdatedAt
		}
	}
	if !latestUpdate.IsZero() {
		latestUpdateStr = latestUpdate.Format("2006-01-02T15:04:05-07:00")
	}

	resp := map[string]interface{}{
		"symbols":    snapshots,
		"count":      len(snapshots),
		"updated_at": latestUpdateStr,
	}

	// 先用 json.Marshal 完整序列化再写入，避免 Encode 中途失败时返回截断 JSON
	w.Header().Set("Content-Type", "application/json")
	body, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[snapshot_handler] json marshal failed: %v", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}
