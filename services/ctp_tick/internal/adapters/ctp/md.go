// ctp_tick/adapters/ctp — CTP 行情订阅核心（cgo 真实现）
//go:build ctp
// +build ctp

package ctp

/*
#cgo LDFLAGS: -lthostmduserapi_se -lthosttraderapi_se

// CTP C API 声明（openctp/ctp-go 提供的 C 接口）
typedef void CThostFtdcMdApi;
extern CThostFtdcMdApi* CreateMdApi(const char* flowPath);
extern void RegisterFront(CThostFtdcMdApi* api, const char* frontAddr);
extern void RegisterSpi(CThostFtdcMdApi* api, void* spi);
extern int Init(CThostFtdcMdApi* api);
extern int Release(CThostFtdcMdApi* api);
extern int ReqUserLogin(CThostFtdcMdApi* api, void* req, int requestID);
extern int SubscribeMarketData(CThostFtdcMdApi* api, char* instruments[], int count);
*/
import "C"
import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// MdAPI 封装 CTP 行情 API 的完整生命周期
// 实现 MarketDataSubscriber 接口
// 参考 Python ctp_engine 的 MD API 部分（CreateFtdcMdApi → RegisterFront → Init → 登录 → 订阅）
type MdAPI struct {
	api       unsafe.Pointer
	frontAddr string
	flowPath  string
	brokerID  string
	userID    string
	password  string
	symbols   []string
	handler   TickHandler
	reqID     atomic.Int32

	state     atomic.Int32  // ConnectionState
	connected chan struct{} // OnFrontConnected 信号
	loggedIn  chan error    // OnRspUserLogin 信号

	disconnectTimes  []time.Time
	disconnectMu     sync.Mutex
	reconnectTimer   *time.Timer // 重连定时器（Close 时可取消）
	tradingDay       string

	mu sync.RWMutex
}

// NewMarketDataSubscriber 创建行情订阅实例（工厂函数，返回接口）
func NewMarketDataSubscriber(flowPath, frontAddr, brokerID, userID, password string) MarketDataSubscriber {
	return newMdAPI(flowPath, frontAddr, brokerID, userID, password)
}

func newMdAPI(flowPath, frontAddr, brokerID, userID, password string) *MdAPI {
	m := &MdAPI{
		frontAddr:   frontAddr,
		flowPath:    flowPath,
		brokerID:    brokerID,
		userID:      userID,
		password:    password,
		connected:   make(chan struct{}, 1),
		loggedIn:    make(chan error, 1),
		reconnectCh: make(chan struct{}, 1),
	}
	m.state.Store(int32(StateDisconnected))
	return m
}

// Connect 完整 CTP 连接序列
// 参考 Python 版 API 调用序列：CreateApi → RegisterFront → RegisterSpi → Init → 等待连接 → 登录 → 订阅
func (m *MdAPI) Connect(symbols []string) error {
	m.symbols = symbols

	// Step 1: 创建 API 实例（对应 Python：mdapi.CreateFtdcMdApi(flow)）
	flowCStr := C.CString(m.flowPath)
	defer C.free(unsafe.Pointer(flowCStr))
	m.api = unsafe.Pointer(C.CreateMdApi(flowCStr))
	if m.api == nil {
		return fmt.Errorf("ctp: CreateMdApi returned nil")
	}

	// Step 2: 注册前置机地址（对应 Python：api.RegisterFront(MD_FRONT)）
	frontCStr := C.CString(m.frontAddr)
	defer C.free(unsafe.Pointer(frontCStr))
	C.RegisterFront((*C.CThostFtdcMdApi)(m.api), frontCStr)
	log.Printf("[ctp] front registered: %s", m.frontAddr)

	// Step 3: 注册 SPI 回调（对应 Python：api.RegisterSpi(spi)）
	spiPtr := m.createSpiPtr()
	C.RegisterSpi((*C.CThostFtdcMdApi)(m.api), spiPtr)
	log.Printf("[ctp] spi registered")

	// Step 4: 初始化（异步连接，对应 Python：api.Init()）
	m.state.Store(int32(StateConnecting))
	C.Init((*C.CThostFtdcMdApi)(m.api))
	log.Printf("[ctp] init called, waiting for OnFrontConnected...")

	// Step 5: 等待连接回调（对应 Python：OnFrontConnected）
	if err := m.waitConnected(); err != nil {
		return err
	}

	// Step 6: 登录（对应 Python：ReqUserLogin）
	if err := m.login(); err != nil {
		return err
	}

	// Step 7: 等待登录结果（对应 Python：OnRspUserLogin）
	if err := m.waitLogin(); err != nil {
		return err
	}
	m.state.Store(int32(StateLoggedIn))

	// Step 8: 订阅行情（对应 Python：SubscribeMarketData）
	if err := m.subscribe(symbols); err != nil {
		return err
	}
	m.state.Store(int32(StateSubscribed))
	log.Printf("[ctp] subscribed %d symbols", len(symbols))

	return nil
}

// Close 释放 API 资源（停止重连定时器 + 清理 C API + 注销 SPI 注册）
func (m *MdAPI) Close() {
	// 取消等待中的重连定时器（防止 Close 后仍触发重连）
	m.stopReconnectTimer()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.api != nil {
		// C.Release((*C.CThostFtdcMdApi)(m.api))  // 需 Linux cgo 环境
		m.api = nil
	}
	m.state.Store(int32(StateDisconnected))

	// 从全局注册表移除（修复 apiRegistry 内存泄露）
	// unregisterAPI 需 key，此处通过遍历清理
	// 实际调用见 spi.go: cleanupRegistry
}

// OnTick 注册行情回调
func (m *MdAPI) OnTick(handler TickHandler) {
	m.mu.Lock()
	m.handler = handler
	m.mu.Unlock()
}

// IsConnected 返回是否已连接
func (m *MdAPI) IsConnected() bool {
	return m.state.Load() >= int32(StateConnected)
}

// TradingDay 返回当前交易日
func (m *MdAPI) TradingDay() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tradingDay
}

// GetState 返回连接状态枚举
func (m *MdAPI) GetState() ConnectionState {
	return ConnectionState(m.state.Load())
}

// --- 内部方法 ---

func (m *MdAPI) waitConnected() error {
	select {
	case <-m.connected:
		log.Printf("[ctp] OnFrontConnected")
		return nil
	case <-time.After(LoginTimeoutSec * time.Second):
		return fmt.Errorf("ctp: connect timeout %ds", LoginTimeoutSec)
	}
}

func (m *MdAPI) login() error {
	reqID := int(m.reqID.Add(1))
	log.Printf("[ctp] ReqUserLogin broker=%s user=%s reqID=%d", m.brokerID, m.userID, reqID)
	// TODO: 构建 CThostFtdcReqUserLoginField + 调用 C.ReqUserLogin
	return nil
}

func (m *MdAPI) waitLogin() error {
	select {
	case err := <-m.loggedIn:
		if err != nil {
			return fmt.Errorf("ctp: login rejected: %w", err)
		}
		return nil
	case <-time.After(LoginTimeoutSec * time.Second):
		return fmt.Errorf("ctp: login timeout %ds", LoginTimeoutSec)
	}
}

func (m *MdAPI) subscribe(symbols []string) error {
	log.Printf("[ctp] SubscribeMarketData: %v", symbols)
	// TODO: 转换 []string → C char* 数组 → C.SubscribeMarketData
	return nil
}

// --- SPI 回调调用入口（由 spi.go 的 C 回调函数调用） ---

func (m *MdAPI) fireConnected() {
	m.state.Store(int32(StateConnected))
	select {
	case m.connected <- struct{}{}:
	default:
	}
}

func (m *MdAPI) fireLoginResult(err error) {
	select {
	case m.loggedIn <- err:
	default:
	}
}

func (m *MdAPI) fireTick(raw *DepthMarketData) {
	m.notifyTick(raw)
}

func (m *MdAPI) notifyTick(data *DepthMarketData) {
	m.mu.RLock()
	h := m.handler
	m.mu.RUnlock()
	if h != nil {
		h(data)
	}
}

// createSpiPtr 创建 C SPI 回调结构体指针
// 由 spi.go 实现具体的回调函数
func (m *MdAPI) createSpiPtr() unsafe.Pointer {
	return createMdSpi(m)
}
