// ctp_tick/api — Tick 数据注入端点（测试用）
//
// 用途：允许外部脚本通过 HTTP POST 向 ctp_tick 注入模拟 Tick 数据。
// 注入的 Tick 走完整内部管道：normalizer → tickWriter（异步 + 批量 + focus）→ Redis。
// 与真实 CTP 回调路径完全一致，仅数据来源不同。
//
// 端口通过 INGEST_PORT 环境变量配置（默认 8090），不与 HEALTH_PORT 冲突。
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

// ============================================================
// IngestHandler — 测试数据注入处理器
// ============================================================

// IngestDeps 注入端点所需的依赖
type IngestDeps struct {
	TickWriter     TickWriterInterface
	LastPriceStore *domain.LastPriceStore
	LastTickTime   *atomic.Pointer[time.Time]
	Metrics        *Metrics
}

// TickWriterInterface ctp_tick writer 的接口约定
// adapters/redis.TickWriter 实现了此接口
type TickWriterInterface interface {
	Send(symbol string, tick *domain.StandardTick) bool
}

// NewIngestHandler 创建注入端点 handler
func NewIngestHandler(deps *IngestDeps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed, use POST"}`, http.StatusMethodNotAllowed)
			return
		}

		// 解析请求体
		var req ingestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ingestResponse{
				Status:   "error",
				Error:    "invalid JSON: " + err.Error(),
				Ingested: 0,
			})
			return
		}

		// 单条或批量
		if len(req.Ticks) == 0 {
			// 单条模式：body 本身就是一个 tick
			single, ok := singleTickFromRequest(&req)
			if !ok {
				writeJSON(w, http.StatusBadRequest, ingestResponse{
					Status:   "error",
					Error:    "no tick data: provide 'ticks' array or single tick fields",
					Ingested: 0,
				})
				return
			}
			req.Ticks = []ingestTick{single}
		}

		// 逐条处理
		ingested, rejected := 0, 0
		for i := range req.Ticks {
			t := &req.Ticks[i]
			if t.Symbol == "" {
				rejected++
				continue
			}
			if t.LastPrice <= 0 || t.LastPrice > domain.MaxValidPrice {
				rejected++
				continue
			}

			raw := tickToRawMarketData(t)
			tick, err := domain.FromRawMarketData(raw)
			if err != nil {
				rejected++
				continue
			}

			// 更新内存快照
			deps.LastPriceStore.Update(tick)

			// 非阻塞发送到 writer goroutine
			if deps.TickWriter.Send(tick.Symbol, tick) {
				deps.Metrics.RedisWrites.Add(1)
				ingested++
			} else {
				deps.Metrics.RedisFailures.Add(1)
				rejected++
			}

			deps.Metrics.TickReceived.Add(1)

			// 更新最后 Tick 时间
			now := time.Now()
			deps.LastTickTime.Store(&now)
		}

		log.Printf("[ingest] received=%d ingested=%d rejected=%d", len(req.Ticks), ingested, rejected)

		writeJSON(w, http.StatusOK, ingestResponse{
			Status:   "ok",
			Ingested: ingested,
			Rejected: rejected,
			Total:    len(req.Ticks),
		})
	})

	return mux
}

// ============================================================
// 请求/响应结构
// ============================================================

// ingestRequest 单条或批量注入请求
// 单条模式下，外层字段即为一条 tick
// 批量模式下，ticks 数组包含多条
type ingestRequest struct {
	// 单条字段（与 ticks 互斥）
	Symbol       string  `json:"symbol"`
	LastPrice    float64 `json:"last_price"`
	Volume       int64   `json:"volume"`
	OpenPrice    float64 `json:"open_price"`
	HighPrice    float64 `json:"high_price"`
	LowPrice     float64 `json:"low_price"`
	PreClose     float64 `json:"pre_close"`
	PreSettle    float64 `json:"pre_settle"`
	OpenInterest float64 `json:"open_interest"`
	UpperLimit   float64 `json:"upper_limit"`
	LowerLimit   float64 `json:"lower_limit"`
	BidPrice1    float64 `json:"bid_price1"`
	BidVolume1   int64   `json:"bid_volume1"`
	AskPrice1    float64 `json:"ask_price1"`
	AskVolume1   int64   `json:"ask_volume1"`
	UpdateTime   string  `json:"update_time"`

	// 批量字段
	Ticks []ingestTick `json:"ticks"`
}

// ingestTick 单条 tick 数据
type ingestTick struct {
	Symbol       string  `json:"symbol"`
	LastPrice    float64 `json:"last_price"`
	Volume       int64   `json:"volume"`
	OpenPrice    float64 `json:"open_price"`
	HighPrice    float64 `json:"high_price"`
	LowPrice     float64 `json:"low_price"`
	PreClose     float64 `json:"pre_close"`
	PreSettle    float64 `json:"pre_settle"`
	OpenInterest float64 `json:"open_interest"`
	UpperLimit   float64 `json:"upper_limit"`
	LowerLimit   float64 `json:"lower_limit"`
	BidPrice1    float64 `json:"bid_price1"`
	BidVolume1   int64   `json:"bid_volume1"`
	AskPrice1    float64 `json:"ask_price1"`
	AskVolume1   int64   `json:"ask_volume1"`
	UpdateTime   string  `json:"update_time"`
}

// ingestResponse 注入结果
type ingestResponse struct {
	Status   string `json:"status"`
	Ingested int    `json:"ingested"`
	Rejected int    `json:"rejected"`
	Total    int    `json:"total,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ============================================================
// 转换函数
// ============================================================

// singleTickFromRequest 从请求外层字段构建 ingestTick
func singleTickFromRequest(req *ingestRequest) (ingestTick, bool) {
	if req.Symbol == "" || req.LastPrice <= 0 {
		return ingestTick{}, false
	}
	return ingestTick{
		Symbol:       req.Symbol,
		LastPrice:    req.LastPrice,
		Volume:       req.Volume,
		OpenPrice:    req.OpenPrice,
		HighPrice:    req.HighPrice,
		LowPrice:     req.LowPrice,
		PreClose:     req.PreClose,
		PreSettle:    req.PreSettle,
		OpenInterest: req.OpenInterest,
		UpperLimit:   req.UpperLimit,
		LowerLimit:   req.LowerLimit,
		BidPrice1:    req.BidPrice1,
		BidVolume1:   req.BidVolume1,
		AskPrice1:    req.AskPrice1,
		AskVolume1:   req.AskVolume1,
		UpdateTime:   req.UpdateTime,
	}, true
}

// tickToRawMarketData 将 ingestTick 转为 domain.RawMarketData
func tickToRawMarketData(t *ingestTick) *domain.RawMarketData {
	updateTime := t.UpdateTime
	if updateTime == "" {
		updateTime = time.Now().Format(domain.TimeFormatCTP)
	}
	tradingDay := time.Now().Format(domain.TradingDayLayout)

	return &domain.RawMarketData{
		TradingDay:         tradingDay,
		InstrumentID:       t.Symbol,
		UpdateTime:         updateTime,
		UpdateMillisec:     0,
		LastPrice:          t.LastPrice,
		PreSettlementPrice: zeroFloatIfEmpty(t.PreSettle),
		PreClosePrice:      zeroFloatIfEmpty(t.PreClose),
		OpenPrice:          zeroFloatIfEmpty(t.OpenPrice),
		HighestPrice:       zeroFloatIfEmpty(t.HighPrice),
		LowestPrice:        zeroFloatIfEmpty(t.LowPrice),
		Volume:             t.Volume,
		OpenInterest:       t.OpenInterest,
		UpperLimitPrice:    zeroFloatIfEmpty(t.UpperLimit),
		LowerLimitPrice:    zeroFloatIfEmpty(t.LowerLimit),
		BidPrice1:          zeroFloatIfEmpty(t.BidPrice1),
		BidVolume1:         t.BidVolume1,
		AskPrice1:          zeroFloatIfEmpty(t.AskPrice1),
		AskVolume1:         t.AskVolume1,
	}
}

func zeroFloatIfEmpty(v float64) float64 {
	return v
}

// writeJSON 写 JSON 响应
func writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}
