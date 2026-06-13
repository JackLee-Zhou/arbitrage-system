// trading_agent/domain/decision — 决策融合
// 注意：本包零外部依赖，只做纯数学计算
package decision

import "math"

// FusedDecision 融合后的最终决策
type FusedDecision struct {
	Action       string  // "buy" / "sell" / "hold"
	PositionSize int     // 建议仓位（手）
	FinalBias    float64 // 最终偏差值
	Confidence   float64 // 综合置信度
}

// Fuse 决策融合公式
// finalBias = score × confidence × decay × alpha
// positionSize = technicalSignal.strength × (1 + finalBias)
func Fuse(aiScore, aiConfidence, decay, alpha, techStrength float64, techAction string) *FusedDecision {
	finalBias := aiScore * aiConfidence * decay * alpha
	adjustedStrength := techStrength * (1 + finalBias)
	// 钳制在 [-1, 2]
	adjustedStrength = math.Max(-1, math.Min(2, adjustedStrength))

	action := techAction
	if math.Abs(finalBias) > 0.3 && techAction == "hold" {
		if finalBias > 0 {
			action = "buy"
		} else {
			action = "sell"
		}
	}

	positionSize := int(math.Round(math.Abs(adjustedStrength)))
	if positionSize == 0 {
		action = "hold"
	}

	return &FusedDecision{
		Action:       action,
		PositionSize: positionSize,
		FinalBias:    finalBias,
		Confidence:   aiConfidence * math.Abs(techStrength),
	}
}

// DecayScore 见 decay.go — AI 评分老化系数（硬编码，不可配置）
