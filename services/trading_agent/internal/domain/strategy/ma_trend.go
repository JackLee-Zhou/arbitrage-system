// trading_agent/domain/strategy — MA双均线趋势跟随策略（V1 唯一策略）
// 注意：本包零外部依赖，只做纯信号计算
//
// TODO(M10): MATrendStrategy 为 V1 硬编码策略。P3 阶段用户可通过 WebUI 上传新策略 YAML，
//
//	引擎从 Redis 读取配置后动态实例化。但 Strategy 接口（Name + Evaluate）保持不变，
//	新策略只需实现该接口即可被引擎加载。
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

// Name 返回策略名称
func (s *MATrendStrategy) Name() string { return "ma_trend" }

// Evaluate 评估双均线交叉信号（实现 Strategy 接口）
func (s *MATrendStrategy) Evaluate(data StrategyInput) *Signal {
	fastMA := data.FastMA
	slowMA := data.SlowMA

	if len(fastMA) < 2 || len(slowMA) < 2 {
		return &Signal{Symbol: data.Symbol, Action: "hold", Reason: "not enough data"}
	}

	curr := fastMA[len(fastMA)-1] - slowMA[len(slowMA)-1]
	prev := fastMA[len(fastMA)-2] - slowMA[len(slowMA)-2]

	if prev <= 0 && curr > 0 {
		return &Signal{Symbol: data.Symbol, Action: "buy", Strength: 0.6, Reason: "golden_cross"}
	}
	if prev >= 0 && curr < 0 {
		return &Signal{Symbol: data.Symbol, Action: "sell", Strength: 0.6, Reason: "death_cross"}
	}

	// 趋势持续信号
	if curr > 0 {
		return &Signal{Symbol: data.Symbol, Action: "hold", Strength: 0.2, Reason: "above_ma"}
	}
	return &Signal{Symbol: data.Symbol, Action: "hold", Strength: 0.2, Reason: "below_ma"}
}
