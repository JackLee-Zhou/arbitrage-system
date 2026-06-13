// trading_agent/domain/indicator — 布林带计算
package indicator

import "math"

// BollingerBand 布林带
type BollingerBand struct {
	Upper float64
	Mid   float64
	Lower float64
	Width float64 // (Upper - Lower) / Mid
}

// Bollinger 计算布林带
// period: 周期（默认 20），stdDev: 标准差倍数（默认 2.0）
func Bollinger(prices []float64, period int, stdDev float64) *BollingerBand {
	if len(prices) < period {
		return nil
	}
	recent := prices[len(prices)-period:]
	mean := 0.0
	for _, p := range recent {
		mean += p
	}
	mean /= float64(period)

	variance := 0.0
	for _, p := range recent {
		variance += (p - mean) * (p - mean)
	}
	variance /= float64(period)
	std := math.Sqrt(variance)

	return &BollingerBand{
		Upper: mean + stdDev*std,
		Mid:   mean,
		Lower: mean - stdDev*std,
		Width: (2 * stdDev * std) / mean,
	}
}
