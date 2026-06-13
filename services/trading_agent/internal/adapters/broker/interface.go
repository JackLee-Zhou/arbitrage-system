// trading_agent/adapters/broker — Broker 抽象接口
// ⭐⭐ 为未来股票接入预留的扩展点
package broker

// OrderRequest 报单请求
type OrderRequest struct {
	Symbol    string
	Direction string // "buy" / "sell"
	Offset    string // "open" / "close" / "close_today"
	Volume    int32
	Price     float64 // 0 = 市价单
	OrderType string  // "limit" / "market"
}

// OrderResponse 报单响应
type OrderResponse struct {
	OrderID      string
	Status       string // "submitted" / "filled" / "cancelled" / "rejected"
	FilledPrice  float64
	FilledVolume int32
	Message      string
}

// Tick 行情 Tick
type Tick struct {
	Symbol    string
	Timestamp int64
	LastPrice float64
	Volume    int64
}

// Position 持仓
type Position struct {
	Symbol    string
	Direction string
	Volume    int32
	AvgPrice  float64
}

// Broker 交易通道抽象接口
// V1 实现：CTPBroker；V2 扩展：QMTBroker / EasyTraderBroker
type Broker interface {
	// 行情订阅
	Subscribe(symbols []string) error

	// 报单操作
	PlaceOrder(req OrderRequest) (OrderResponse, error)
	CancelOrder(orderID string) error

	// 查询
	QueryPositions() ([]Position, error)
	QueryAccount() (interface{}, error)

	// 行情回调注册
	OnTick(handler func(Tick))
}
