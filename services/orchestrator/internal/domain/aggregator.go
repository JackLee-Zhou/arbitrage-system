// orchestrator/domain — 状态聚合 + 指令路由（纯逻辑）
package domain

// Overview 系统概览（聚合 Redis + trading_agent 状态）
type Overview struct {
	EngineStatus string      `json:"engine_status"`
	Symbol       string      `json:"symbol"`
	LastPrice    float64     `json:"last_price"`
	Position     interface{} `json:"position"`
	AIScore      float64     `json:"ai_score"`
	AIScoreAge   int64       `json:"ai_score_age_sec"`
	OpenOrders   int         `json:"open_orders"`
	DailyPnL     float64     `json:"daily_pnl"`
}
