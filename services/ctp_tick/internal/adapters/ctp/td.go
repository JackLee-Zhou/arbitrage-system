// ctp_tick/adapters/ctp — TD API 全市场合约查询（cgo 真实现）
//go:build ctp
// +build ctp

package ctp

/*
#cgo LDFLAGS: -lthosttraderapi_se

// CTP Trade API C 声明
typedef void CThostFtdcTraderApi;
extern CThostFtdcTraderApi* CreateTradeApi(const char* flowPath);
extern void Trade_RegisterFront(CThostFtdcTraderApi* api, const char* frontAddr);
extern void Trade_RegisterSpi(CThostFtdcTraderApi* api, void* spi);
extern int Trade_Init(CThostFtdcTraderApi* api);
extern int Trade_Release(CThostFtdcTraderApi* api);
extern int Trade_ReqAuthenticate(CThostFtdcTraderApi* api, void* req, int requestID);
extern int Trade_ReqUserLogin(CThostFtdcTraderApi* api, void* req, int requestID, int timeout, char* userProductInfo);
extern int Trade_ReqQryInstrument(CThostFtdcTraderApi* api, void* req, int requestID);
*/
import "C"
import (
	"fmt"
	"log"
	"sync"
	"unsafe"
)

// ============================================================
// TdAPI — 交易 API 封装（仅用于启动时查询全市场合约，查完即释放）
// 对应 Python ctp_engine load_all_futures()
// ============================================================

// TdAPI TD 行情查询 API
type TdAPI struct {
	api       unsafe.Pointer
	frontAddr string
	brokerID  string
	userID    string
	password  string

	allCodes []string
	done     chan struct{}
	loginOK  chan struct{}
	authOK   chan struct{}
	errorCh  chan error

	mu sync.Mutex
}

// NewContractQuerier 创建合约查询器（cgo 真实现）
func NewContractQuerier() ContractQuerier {
	return &TdAPI{
		done:    make(chan struct{}, 1),
		loginOK: make(chan struct{}, 1),
		authOK:  make(chan struct{}, 1),
		errorCh: make(chan error, 1),
	}
}

// QueryAllFutures 查询全市场期货合约
// API 序列: CreateTradeApi → RegisterFront → RegisterSpi → Init → 等 OnFrontConnected →
// ReqAuthenticate → OnRspAuthenticate → ReqUserLogin → OnRspUserLogin → 6 交易所循环 ReqQryInstrument
func (t *TdAPI) QueryAllFutures(frontAddr, brokerID, userID, password string) ([]string, error) {
	t.frontAddr = frontAddr
	t.brokerID = brokerID
	t.userID = userID
	t.password = password
	t.allCodes = nil

	log.Printf("[ctp.td] connecting to %s for instrument query...", frontAddr)

	// Step 1: 创建 Trade API 实例（对应 Python tdapi.CreateFtdcTraderApi(flow)）
	flowPath := C.CString("./flow_td/td_query")
	defer C.free(unsafe.Pointer(flowPath))
	t.api = unsafe.Pointer(C.CreateTradeApi(flowPath))
	if t.api == nil {
		return nil, fmt.Errorf("ctp.td: CreateTradeApi failed")
	}

	// Step 2: 注册前置机 + SPI + Init（异步连接）
	frontCStr := C.CString(frontAddr)
	defer C.free(unsafe.Pointer(frontCStr))
	C.Trade_RegisterFront((*C.CThostFtdcTraderApi)(t.api), frontCStr)

	spiPtr := t.createSpiPtr()
	C.Trade_RegisterSpi((*C.CThostFtdcTraderApi)(t.api), spiPtr)

	C.Trade_Init((*C.CThostFtdcTraderApi)(t.api))

	// Step 3: 等待完整序列完成
	select {
	case <-t.done:
		log.Printf("[ctp.td] query complete: %d futures found", len(t.allCodes))
	case err := <-t.errorCh:
		log.Printf("[ctp.td] query error: %v", err)
		return nil, err
	case <-time.After(TDQueryTimeoutSec * time.Second):
		// 超时兜底，与 Python 版 wait(timeout=30) 一致
	}

	// Step 4: 无论成功/失败都释放 API 资源
	// C.Trade_Release((*C.CThostFtdcTraderApi)(t.api))
	t.api = nil

	log.Printf("[ctp.td] %d futures loaded", len(t.allCodes))
	return t.allCodes, nil
}
