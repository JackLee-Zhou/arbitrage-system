// trading_agent/domain/indicator — 技术指标计算
// 注意：本包零外部依赖，只做纯数学计算
package indicator

// SMA 简单移动平均
func SMA(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil
	}
	result := make([]float64, len(prices)-period+1)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	result[0] = sum / float64(period)
	for i := period; i < len(prices); i++ {
		sum += prices[i] - prices[i-period]
		result[i-period+1] = sum / float64(period)
	}
	return result
}

// EMA 指数移动平均
func EMA(prices []float64, period int) []float64 {
	if len(prices) == 0 {
		return nil
	}
	k := 2.0 / float64(period+1)
	result := make([]float64, len(prices))
	result[0] = prices[0]
	for i := 1; i < len(prices); i++ {
		result[i] = prices[i]*k + result[i-1]*(1-k)
	}
	return result
}
