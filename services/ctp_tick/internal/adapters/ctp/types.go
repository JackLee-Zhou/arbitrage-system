// ctp_tick/adapters/ctp — CTP 类型定义 + 常量
// 本文件在所有平台上编译，定义共享类型
package ctp

import "time"

// ============================================================
// CTP 认证常量（参考 openctp SimNow 环境）
// ============================================================
const (
	AppID           = "simnow_client_test"
	AuthCode        = "0000000000000000"
	UserProductInfo = "@openctp"

	LoginTimeoutSec   = 30 // 登录超时（秒）
	DefaultHealthPort = 8081
)

// ============================================================
// 断线重连常量
// ============================================================
const (
	MaxReconnectAttempts = 3  // 滑动窗口内最大重连次数
	ReconnectWindowSec   = 60 // 滑动窗口大小（秒）
	MinReconnectWaitSec  = 10 // 最小重连等待（秒）
	MaxReconnectWaitSec  = 60 // 最大重连等待（秒）
)

// ============================================================
// 注意：MaxValidPrice/MinValidPrice/MaxValidVolume 在 domain/normalizer.go 定义。
// adapters/ctp 不重复定义，使用时引用 domain 包。
// ============================================================
// CThostFtdcDepthMarketDataField Go 侧映射
// CTP C 结构体的字段映射 —— 仅行情订阅需要的字段
// ============================================================

// DepthMarketData CTP 深度行情数据（Go 侧表示）
// 对应 C/C++ 结构体 CThostFtdcDepthMarketDataField
// 字段选取：V1 仅提取 trading_agent 和 indicator 需要的字段 + 5 档盘口
type DepthMarketData struct {
	TradingDay         string  // 交易日
	InstrumentID       string  // 合约代码（如 MA2509）
	UpdateTime         string  // 最后修改时间 HH:mm:ss
	UpdateMillisec     int32   // 最后修改毫秒
	LastPrice          float64 // 最新价
	PreSettlementPrice float64 // 昨结算
	PreClosePrice      float64 // 昨收盘
	OpenPrice          float64 // 开盘价
	HighestPrice       float64 // 最高价
	LowestPrice        float64 // 最低价
	Volume             int64   // 成交量
	Turnover           float64 // 成交金额
	OpenInterest       float64 // 持仓量
	UpperLimitPrice    float64 // 涨停价
	LowerLimitPrice    float64 // 跌停价

	// 5 档买盘
	BidPrice1  float64
	BidVolume1 int64
	BidPrice2  float64
	BidVolume2 int64
	BidPrice3  float64
	BidVolume3 int64
	BidPrice4  float64
	BidVolume4 int64
	BidPrice5  float64
	BidVolume5 int64

	// 5 档卖盘
	AskPrice1  float64
	AskVolume1 int64
	AskPrice2  float64
	AskVolume2 int64
	AskPrice3  float64
	AskVolume3 int64
	AskPrice4  float64
	AskVolume4 int64
	AskPrice5  float64
	AskVolume5 int64
}

// ============================================================
// MarketDataSubscriber 行情订阅抽象接口
// cmd/main.go 通过此接口注入，解耦 cgo 依赖
// ============================================================

// TickHandler 行情回调函数
type TickHandler func(data *DepthMarketData)

// MarketDataSubscriber 行情订阅接口
// 实现：MdAPI（cgo 真实现）/ MockMdAPI（本地开发）
type MarketDataSubscriber interface {
	// Connect 连接前置机 + 登录 + 订阅
	// 内部处理完整的 CTP 序列：RegisterFront → Init → 登录 → 订阅
	Connect(symbols []string) error

	// Close 释放 API 资源
	Close()

	// OnTick 注册行情回调
	OnTick(handler TickHandler)

	// IsConnected 返回连接状态
	IsConnected() bool

	// TradingDay 返回当前交易日
	TradingDay() string
}

// ============================================================
// 连接状态枚举
// ============================================================

// ConnectionState CTP 连接状态
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateLoggedIn
	StateSubscribed
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateLoggedIn:
		return "logged_in"
	case StateSubscribed:
		return "subscribed"
	default:
		return "unknown"
	}
}

// ============================================================
// TD API 全市场合约查询常量
// ============================================================

// 中国六大期货交易所代码（Python ctp_engine 同款列表）
var AllExchanges = []string{"SHFE", "DCE", "CZCE", "CFFEX", "INE", "GFEX"}

const (
	ProductClassFutures = '1' // 期货品种（排除期权/组合/期指令）
	TDQueryTimeoutSec   = 30  // 全市场查询超时（秒）
)

// ============================================================
// ContractQuerier 合约查询抽象接口
// ============================================================

// ContractQuerier 全市场合约查询接口
// 启动时通过 TD API 查询一次，获取所有期货合约列表后释放资源
type ContractQuerier interface {
	// QueryAllFutures 查询 6 个交易所的全市场期货合约
	// 对应 Python ctp_engine 的 load_all_futures()
	QueryAllFutures(frontAddr, brokerID, userID, password string) ([]string, error)
}

// ============================================================
// 异步写入常量
// ============================================================

const (
	TickChannelBufferSize = 10000           // CTP 回调 → writer goroutine 缓冲区
	BatchWriteSize        = 100             // 批量写入阈值（条）
	BatchWriteInterval    = 1 * time.Second // 批量写入间隔
)

// ============================================================
// 定时器常量
// ============================================================

const (
	DaySessionCloseHour   = 15              // 日盘收盘时刻（15:15 后断线）
	NightSessionCloseHour = 23              // 夜盘收盘时刻（23:00 后断线）
	NextDayReconnectHour  = 8               // 次日最早重连时间（8:50 开市前）
	ReconnectLoopInterval = 5 * time.Second // 重连检查间隔
)
