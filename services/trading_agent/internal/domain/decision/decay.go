// trading_agent/domain/decision — AI 评分老化（硬编码，不可配置）
// 老化规则是不可妥协的硬决策，见 ARCHITECTURE.md
package decision

// DecayScore AI 评分老化系数
// < 15min: decay=1.0（完全信任）
// 15-30min: decay=0.5（权重减半）
// > 30min: decay=0.0（盲打模式 + 推钉钉告警）
func DecayScore(ageSeconds int64) float64 {
	switch {
	case ageSeconds < 900: // 15 min
		return 1.0
	case ageSeconds < 1800: // 30 min
		return 0.5
	default:
		return 0.0 // 盲打模式
	}
}

// IsStale 判断评分是否已过期（>30 分钟）
func IsStale(ageSeconds int64) bool {
	return ageSeconds >= 1800
}

// IsDegrading 判断评分是否正在衰减（15-30 分钟）
func IsDegrading(ageSeconds int64) bool {
	return ageSeconds >= 900 && ageSeconds < 1800
}
