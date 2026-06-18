// trading_agent/domain/strategy — 策略引擎调度器
// 注意：本包零外部依赖，只做纯策略调度
//
// TODO(M10): 当前策略通过 engine.Register() 硬编码注册。
//
//	P3 阶段需改为从 Redis 读取 YAML 策略配置列表，按 mode（shadow/live）动态加载：
//	  - shadow 策略：只生成信号，不下单，写 InfluxDB 记录
//	  - live 策略：正常生成信号 + 下单
//	YAML 配置格式见 ARCHITECTURE.md 机制 1。
//	Strategy 接口保持不变，只改 Engine 的加载方式（从 Register 变为 LoadFromConfig）。
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
