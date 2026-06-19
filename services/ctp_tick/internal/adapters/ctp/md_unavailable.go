//go:build !ctp

// ctp_tick/adapters/ctp — CTP 不可用时的报错实现
// 当未使用 -tags ctp 编译时，返回明确错误而非虚假数据。
package ctp

import (
	"errors"
	"fmt"
)

// NewMarketDataSubscriber CTP 不可用时返回错误实现
func NewMarketDataSubscriber(flowPath, frontAddr, brokerID, userID, password string) MarketDataSubscriber {
	return &unavailableAPI{brokerID: brokerID}
}

type unavailableAPI struct {
	brokerID string
}

func (a *unavailableAPI) Connect(symbols []string) error {
	return fmt.Errorf("CTP 不可用：请使用 -tags ctp 编译（当前编译未链接 CTP 库）")
}

func (a *unavailableAPI) Close()                    {}
func (a *unavailableAPI) OnTick(TickHandler)        {}
func (a *unavailableAPI) IsConnected() bool         { return false }
func (a *unavailableAPI) TradingDay() string        { return "" }
func (a *unavailableAPI) GetState() ConnectionState { return StateDisconnected }

// NewContractQuerier CTP 不可用时返回错误实现
func NewContractQuerier() ContractQuerier {
	return &unavailableContractQuerier{}
}

type unavailableContractQuerier struct{}

func (q *unavailableContractQuerier) QueryAllFutures(frontAddr, brokerID, userID, password string) ([]string, error) {
	return nil, errors.New("CTP 不可用：无法查询全市场合约列表，请使用 -tags ctp 编译")
}
