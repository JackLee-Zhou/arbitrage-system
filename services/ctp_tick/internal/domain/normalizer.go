// ctp_tick/domain — Tick 标准化纯逻辑（零外部依赖）
package domain

import (
	"fmt"
	"time"
)

// ============================================================
// 常量（禁止魔法数字 — ENGINEERING_GUIDE.md 规则 6）
// ============================================================
const (
	MaxValidPrice    = 1e100
	MinValidPrice    = 0.0
	MaxValidVolume   = 1e12
	TradingDayLayout = "20060102"
	TimeFormatCTP    = "15:04:05"
)

// ============================================================
// StandardTick — 标准化 Tick（与 contracts/tick.proto Tick 对应）
// ============================================================
type StandardTick struct {
	Symbol       string
	Timestamp    time.Time
	LastPrice    float64
	Volume       int64
	OpenPrice    float64
	HighPrice    float64
	LowPrice     float64
	PreClose     float64
	PreSettle    float64
	OpenInterest float64
	UpperLimit   float64
	LowerLimit   float64
	BidPrice1    float64
	BidVolume1   int64
	AskPrice1    float64
	AskVolume1   int64
}

// ============================================================
// RawMarketData — domain 层的行情输入结构
// 解耦 CTP 具体类型，adapters/ctp 负责将 C 结构体转换为此结构
// ============================================================
type RawMarketData struct {
	TradingDay         string
	InstrumentID       string
	UpdateTime         string
	UpdateMillisec     int32
	LastPrice          float64
	PreSettlementPrice float64
	PreClosePrice      float64
	OpenPrice          float64
	HighestPrice       float64
	LowestPrice        float64
	Volume             int64
	OpenInterest       float64
	UpperLimitPrice    float64
	LowerLimitPrice    float64
	BidPrice1          float64
	BidVolume1         int64
	AskPrice1          float64
	AskVolume1         int64

	// 5 档深度（保留完整数据，StandardTick 仅用第1档）
	BidPrice2  float64
	BidVolume2 int64
	BidPrice3  float64
	BidVolume3 int64
	BidPrice4  float64
	BidVolume4 int64
	BidPrice5  float64
	BidVolume5 int64
	AskPrice2  float64
	AskVolume2 int64
	AskPrice3  float64
	AskVolume3 int64
	AskPrice4  float64
	AskVolume4 int64
	AskPrice5  float64
	AskVolume5 int64
}

// ============================================================
// 标准化转换（参考 Python ctp_engine OnRtnDepthMarketData 的 tick dict 构建）
// ============================================================

// FromRawMarketData 将 CTP 原始行情数据转换为标准化 Tick
// 执行安全过滤、字段映射、时间解析
func FromRawMarketData(raw *RawMarketData) (*StandardTick, error) {
	if raw == nil {
		return nil, fmt.Errorf("invalid market data: nil")
	}
	if !isValidMarketData(raw) {
		return nil, fmt.Errorf("invalid market data: symbol=%s price=%.4f",
			raw.InstrumentID, raw.LastPrice)
	}

	tick := &StandardTick{
		Symbol:       raw.InstrumentID,
		Timestamp:    parseTimestamp(raw.TradingDay, raw.UpdateTime, raw.UpdateMillisec),
		LastPrice:    raw.LastPrice,
		Volume:       raw.Volume,
		OpenPrice:    safePrice(raw.OpenPrice),
		HighPrice:    safePrice(raw.HighestPrice),
		LowPrice:     safePrice(raw.LowestPrice),
		PreClose:     safePrice(raw.PreClosePrice),
		PreSettle:    safePrice(raw.PreSettlementPrice),
		OpenInterest: raw.OpenInterest,
		UpperLimit:   safePrice(raw.UpperLimitPrice),
		LowerLimit:   safePrice(raw.LowerLimitPrice),
		BidPrice1:    safePrice(raw.BidPrice1),
		BidVolume1:   safeVolume(raw.BidPrice1, raw.BidVolume1),
		AskPrice1:    safePrice(raw.AskPrice1),
		AskVolume1:   safeVolume(raw.AskPrice1, raw.AskVolume1),
	}

	return tick, nil
}

// ============================================================
// 数据校验函数
// ============================================================

// isValidMarketData 校验行情有效性
// 对应 Python 版：pData is None 或 LastPrice <= 0 则丢弃
func isValidMarketData(raw *RawMarketData) bool {
	if raw == nil {
		return false
	}
	if raw.InstrumentID == "" {
		return false
	}
	if raw.LastPrice <= MinValidPrice || raw.LastPrice > MaxValidPrice {
		return false
	}
	return true
}

// safePrice 安全价格过滤
// 对应 Python _safe_price()：>0 且 <1e100
func safePrice(price float64) float64 {
	if price > MinValidPrice && price < MaxValidPrice {
		return price
	}
	return 0
}

// safeVolume 安全成交量
// 仅当对应价格有效且量在合理范围内返回有效值
func safeVolume(price float64, volume int64) int64 {
	if price > MinValidPrice && price < MaxValidPrice && volume > 0 && volume < MaxValidVolume {
		return volume
	}
	return 0
}

// ============================================================
// 时间解析
// ============================================================

// parseTimestamp 解析 CTP 交易日 + 更新时间 + 毫秒
// 优先使用组合时间，失败则回退到系统当前时间（与 Python ctp_engine 一致）
func parseTimestamp(tradingDay, updateTime string, millisec int32) time.Time {
	if tradingDay != "" && updateTime != "" {
		if t, err := time.Parse(TradingDayLayout+" "+TimeFormatCTP, tradingDay+" "+updateTime); err == nil {
			return t.Add(time.Duration(millisec) * time.Millisecond)
		}
	}
	return time.Now()
}

// ============================================================
// 序列化辅助 — ProtoTick（供 adapters/redis 序列化用）
// ============================================================

// ProtoTick 与 contracts/tick.proto Tick 消息一致的中间结构
// 等 buf generate 生成 proto 代码后，替换为 generated/go 中的真实类型
type ProtoTick struct {
	Symbol       string  `json:"symbol"`
	Timestamp    int64   `json:"timestamp"`
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
}

// ToProto 将 StandardTick 转换为 ProtoTick（时间戳转 unix ms）
func (t *StandardTick) ToProto() *ProtoTick {
	return &ProtoTick{
		Symbol:       t.Symbol,
		Timestamp:    t.Timestamp.UnixMilli(),
		LastPrice:    t.LastPrice,
		Volume:       t.Volume,
		OpenPrice:    t.OpenPrice,
		HighPrice:    t.HighPrice,
		LowPrice:     t.LowPrice,
		PreClose:     t.PreClose,
		PreSettle:    t.PreSettle,
		OpenInterest: t.OpenInterest,
		UpperLimit:   t.UpperLimit,
		LowerLimit:   t.LowerLimit,
		BidPrice1:    t.BidPrice1,
		BidVolume1:   t.BidVolume1,
		AskPrice1:    t.AskPrice1,
		AskVolume1:   t.AskVolume1,
	}
}
