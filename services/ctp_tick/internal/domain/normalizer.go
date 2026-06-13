// ctp_tick/domain — Tick 标准化纯逻辑（零外部依赖）
package domain

import "time"

// StandardTick 标准化 Tick 结构（与 contracts/tick.proto 对应）
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
