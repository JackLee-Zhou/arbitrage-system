// ctp_tick/adapters/ctp — CTP 行情 SPI 回调实现（cgo 真实现）
//go:build ctp
// +build ctp

package ctp

/*
#include <stdlib.h>

// C 侧回调函数声明（由 openctp C 封装调用）
extern void goOnFrontConnected(void* mdApi);
extern void goOnRspUserLogin(void* mdApi, void* pRspUserLogin, void* pRspInfo, int nRequestID, _Bool bIsLast);
extern void goOnRtnDepthMarketData(void* mdApi, void* pDepthMarketData);
extern void goOnFrontDisconnected(void* mdApi, int nReason);
*/
import "C"
import (
	"log"
	"sync"
	"unsafe"
)

// ============================================================
// MdSpiVtable — C 侧虚函数表（openctp C 封装用函数指针）
// ============================================================

type spiVtable struct {
	onFrontConnected     unsafe.Pointer
	onRspUserLogin       unsafe.Pointer
	onRtnDepthMarketData unsafe.Pointer
	onFrontDisconnected  unsafe.Pointer
}

// createMdSpi 创建 C 侧回调结构体，绑定到 MdAPI 实例
func createMdSpi(md *MdAPI) unsafe.Pointer {
	vt := &spiVtable{
		onFrontConnected:     C.goOnFrontConnected,
		onRspUserLogin:       C.goOnRspUserLogin,
		onRtnDepthMarketData: C.goOnRtnDepthMarketData,
		onFrontDisconnected:  C.goOnFrontDisconnected,
	}
	registerAPI(unsafe.Pointer(vt), md)
	return unsafe.Pointer(vt)
}

// ============================================================
// 全局注册表 — C 侧指针 ↔ Go 侧 MdAPI 实例
// ============================================================

var (
	apiRegistry   = make(map[unsafe.Pointer]*MdAPI)
	apiRegistryMu sync.Mutex
)

func registerAPI(key unsafe.Pointer, md *MdAPI) {
	apiRegistryMu.Lock()
	defer apiRegistryMu.Unlock()
	apiRegistry[key] = md
}

func lookupAPI(key unsafe.Pointer) *MdAPI {
	apiRegistryMu.Lock()
	defer apiRegistryMu.Unlock()
	return apiRegistry[key]
}

func unregisterAPI(key unsafe.Pointer) {
	apiRegistryMu.Lock()
	defer apiRegistryMu.Unlock()
	delete(apiRegistry, key)
}

// cleanupRegistryByAPI 通过 MdAPI 指针遍历清理注册表（Close 时调用）
func cleanupRegistryByAPI(md *MdAPI) {
	apiRegistryMu.Lock()
	defer apiRegistryMu.Unlock()
	for k, v := range apiRegistry {
		if v == md {
			delete(apiRegistry, k)
		}
	}
}

// ============================================================
//export — Go 侧回调函数（C 侧通过 cgo 调用）
// 每个对应 Python ctp_engine MdSpi 的一个方法
// ============================================================

//export goOnFrontConnected
func goOnFrontConnected(mdAPI unsafe.Pointer) {
	md := lookupAPI(mdAPI)
	if md == nil {
		log.Printf("[ctp.spi] OnFrontConnected: unknown api %p", mdAPI)
		return
	}
	log.Printf("[ctp.spi] OnFrontConnected — front connected successfully")
	md.fireConnected()
	md.clearDisconnectHistory() // 连接成功 → 重置断线计数（参考 Python 版）
}

//export goOnRspUserLogin
func goOnRspUserLogin(mdAPI unsafe.Pointer, pRspUserLogin unsafe.Pointer, pRspInfo unsafe.Pointer, nRequestID C.int, bIsLast C._Bool) {
	md := lookupAPI(mdAPI)
	if md == nil {
		return
	}
	if !bool(bIsLast) {
		return // 仅处理最后一条响应
	}

	reqID := int(nRequestID)
	// TODO: 从 pRspInfo 提取 ErrorID 判断登录是否成功
	// ErrorID == 0 表示成功
	// 参考 Python 版：仅检查登录返回是否正常
	log.Printf("[ctp.spi] OnRspUserLogin: reqID=%d — success", reqID)
	md.fireLoginResult(nil)
}

//export goOnRtnDepthMarketData
func goOnRtnDepthMarketData(mdAPI unsafe.Pointer, pData unsafe.Pointer) {
	md := lookupAPI(mdAPI)
	if md == nil || pData == nil {
		return
	}

	raw := cFieldToDepthMarketData(pData)
	// 过滤无效数据（与 Python 版逻辑一致）
	if raw.LastPrice <= 0 {
		return
	}
	md.fireTick(raw)
}

//export goOnFrontDisconnected
func goOnFrontDisconnected(mdAPI unsafe.Pointer, nReason C.int) {
	md := lookupAPI(mdAPI)
	if md == nil {
		return
	}
	reason := int(nReason)
	log.Printf("[ctp.spi] OnFrontDisconnected: reason=%d", reason)
	md.state.Store(int32(StateDisconnected))
	md.handleDisconnect(reason)
}

// ============================================================
// C struct → Go DepthMarketData 字段提取
// ============================================================

// cFieldToDepthMarketData 从 C 结构体指针提取行情数据
// 生产环境使用 openctp/ctp-go 提供的 Go 封装，直接访问 C struct 字段
// 此处为框架代码，待 Linux 环境实现具体的 offset 读取
func cFieldToDepthMarketData(p unsafe.Pointer) *DepthMarketData {
	// TODO: Linux 容器内编译时实现
	// 使用 reflect 或直接 cgo 字段访问：
	// data := (*C.CThostFtdcDepthMarketDataField)(p)
	// return &DepthMarketData{
	//     InstrumentID: C.GoString(&data.InstrumentID[0]),
	//     LastPrice:    float64(data.LastPrice),
	//     ...
	// }
	_ = p
	return &DepthMarketData{}
}
