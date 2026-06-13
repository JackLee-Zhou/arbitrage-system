// trading_agent/domain/strategy — 策略引擎
// 注意：本包零外部依赖，只做纯信号计算
package strategy

// Signal 策略信号
type Signal struct {
	Symbol    string
	Action    string  // "buy" / "sell" / "hold"
	Strength  float64 // [0, 1] 信号强度
	Reason    string
	Timestamp int64
}

// MATrendStrategy 双均线趋势跟随策略
type MATrendStrategy struct {
	FastMA int // 快线周期（默认 5）
	SlowMA int // 慢线周期（默认 20）
}

// Evaluate 评估双均线交叉信号
// fastMA, slowMA 为最新的均线值数组
func (s *MATrendStrategy) Evaluate(fastMA, slowMA []float64) *Signal {
	if len(fastMA) < 2 || len(slowMA) < 2 {
		return &Signal{Action: "hold", Reason: "not enough data"}
	}

	curr := fastMA[len(fastMA)-1] - slowMA[len(slowMA)-1]
	prev := fastMA[len(fastMA)-2] - slowMA[len(slowMA)-2]

	if prev <= 0 && curr > 0 {
		return &Signal{Action: "buy", Strength: 0.6, Reason: "golden_cross"}
	}
	if prev >= 0 && curr < 0 {
		return &Signal{Action: "sell", Strength: 0.6, Reason: "death_cross"}
	}

	// 趋势持续信号
	if curr > 0 {
		return &Signal{Action: "hold", Strength: 0.2, Reason: "above_ma"}
	}
	return &Signal{Action: "hold", Strength: 0.2, Reason: "below_ma"}
}
