// trading_agent/domain/indicator — 量比 / 动量指标
package indicator

// VolumeRatio 量比 = 最近 N 根 K 线平均成交量 / 过去 M 根 K 线平均成交量
func VolumeRatio(volumes []int64, recentN, pastM int) float64 {
	if len(volumes) < recentN+pastM {
		return 1.0
	}
	recent := volumes[len(volumes)-recentN:]
	past := volumes[len(volumes)-recentN-pastM : len(volumes)-recentN]

	recentAvg := avgVolume(recent)
	pastAvg := avgVolume(past)
	if pastAvg == 0 {
		return 1.0
	}
	return recentAvg / pastAvg
}

// Momentum 价格动量 = (当前价 / N 根前价 - 1) * 100
func Momentum(prices []float64, period int) []float64 {
	if len(prices) < period+1 {
		return nil
	}
	result := make([]float64, len(prices)-period)
	for i := period; i < len(prices); i++ {
		result[i-period] = (prices[i]/prices[i-period] - 1) * 100
	}
	return result
}

// OneSidedRatio 单向成交比 = max(主动买, 主动卖) / 总成交量
func OneSidedRatio(bidVolume, askVolume int64) float64 {
	total := bidVolume + askVolume
	if total == 0 {
		return 0.5
	}
	maxVol := bidVolume
	if askVolume > maxVol {
		maxVol = askVolume
	}
	return float64(maxVol) / float64(total)
}

func avgVolume(volumes []int64) float64 {
	if len(volumes) == 0 {
		return 0
	}
	sum := int64(0)
	for _, v := range volumes {
		sum += v
	}
	return float64(sum) / float64(len(volumes))
}
