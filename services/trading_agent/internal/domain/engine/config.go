// trading_agent/domain/engine — 引擎配置
package engine

import "time"

// EngineConfig 引擎配置参数
type EngineConfig struct {
	Symbol            string        // 合约代码，如 "MA2509"
	AIAlpha           float64       // AI 评分参与权重，默认 0.2
	HistorySize       int           // 价格缓冲区容量，默认 200
	MaxPosition       int           // 最大持仓手数，默认 2
	DailyMaxLossPct   float64       // 日最大亏损比例，默认 0.02
	StopLossPct       float64       // 单笔止损比例，默认 0.005
	EmergencyCooldown time.Duration // 紧急触发防抖间隔，默认 5 分钟
}

// DefaultConfig 返回默认引擎配置
func DefaultConfig() *EngineConfig {
	return &EngineConfig{
		Symbol:            "", // 必须从环境变量 SYMBOL 设置
		AIAlpha:           0.2,
		HistorySize:       200,
		MaxPosition:       2,
		DailyMaxLossPct:   0.02,
		StopLossPct:       0.005,
		EmergencyCooldown: 5 * time.Minute,
	}
}
