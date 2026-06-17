// trading_agent/domain/indicator — ATR（平均真实波幅）
// TODO(M10): 改为 ATR 实现 Indicator 接口，支持从 YAML 配置动态加载参数。
//
//	见 ma.go 顶部 TODO 说明。
package indicator

import "math"

// TrueRange 单根 K 线真实波幅
func TrueRange(high, low, prevClose float64) float64 {
	tr1 := high - low
	tr2 := math.Abs(high - prevClose)
	tr3 := math.Abs(low - prevClose)
	return math.Max(tr1, math.Max(tr2, tr3))
}

// ATR 平均真实波幅（Wilder's smoothing）
func ATR(highs, lows, closes []float64, period int) []float64 {
	if len(highs) < period+1 {
		return nil
	}
	n := len(highs)
	tr := make([]float64, n)
	for i := 1; i < n; i++ {
		tr[i] = TrueRange(highs[i], lows[i], closes[i-1])
	}

	result := make([]float64, n-period)
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += tr[i]
	}
	result[0] = sum / float64(period)
	for i := period + 1; i < n; i++ {
		result[i-period] = (result[i-period-1]*float64(period-1) + tr[i]) / float64(period)
	}
	return result
}
