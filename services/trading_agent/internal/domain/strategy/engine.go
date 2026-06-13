// trading_agent/domain/strategy — 策略引擎调度器
// 注意：本包零外部依赖，只做纯策略调度
package strategy

// Engine 策略引擎
type Engine struct {
	Strategies []Strategy
}

// Strategy 策略接口
type Strategy interface {
	Name() string
	Evaluate(data StrategyInput) *Signal
}

// StrategyInput 策略输入数据
type StrategyInput struct {
	Symbol    string
	FastMA    []float64
	SlowMA    []float64
	BollUpper float64
	BollLower float64
	ATR       float64
	LastPrice float64
	Volume    int64
}

// NewEngine 创建策略引擎
func NewEngine() *Engine {
	return &Engine{
		Strategies: make([]Strategy, 0),
	}
}

// Register 注册策略
func (e *Engine) Register(s Strategy) {
	e.Strategies = append(e.Strategies, s)
}

// Run 运行所有已注册策略，返回信号列表
func (e *Engine) Run(input StrategyInput) []*Signal {
	var signals []*Signal
	for _, s := range e.Strategies {
		sig := s.Evaluate(input)
		if sig != nil && sig.Action != "hold" {
			signals = append(signals, sig)
		}
	}
	return signals
}
