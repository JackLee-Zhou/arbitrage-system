// ctp_tick/adapters/ctp — Mock 合约查询（macOS 本地开发用）
//go:build !ctp

package ctp

import "log"

// ============================================================
// MockTdAPI — 返回硬编码期货合约列表
// ============================================================

// mockFutures 模拟全市场期货合约列表
// 以化工类为主 + 常见主力合约，模拟真实市场 ~330 个合约
var mockFutures = []string{
	// 郑商所 CZCE — 化工 + 农产品
	"MA509", "MA601", "TA509", "TA601", "FG509", "FG601",
	"SA509", "SA601", "UR509", "UR601", "PF509", "PF601",
	"CF509", "SR509", "OI509", "RM509",

	// 大商所 DCE — 化工 + 黑色 + 农产品
	"pp2509", "pp2601", "l2509", "l2601", "v2509", "v2601",
	"eb2509", "eb2601", "eg2509", "eg2601", "pg2509", "pg2601",
	"i2509", "j2509", "jm2509", "m2509", "y2509", "a2509",

	// 上期所 SHFE — 有色 + 黑色 + 能化
	"rb2510", "hc2510", "cu2507", "al2507", "zn2507",
	"ni2507", "sn2507", "pb2507", "ao2507",
	"ru2509", "fu2509", "bu2506", "sp2509",

	// 能源中心 INE
	"sc2507", "lu2508", "bc2507", "nr2509",

	// 中金所 CFFEX — 股指 + 国债
	"IF2409", "IC2409", "IH2409", "IM2409",

	// 广期所 GFEX
	"si2509", "lc2509",
}

// MockTdAPI Mock 合约查询器（本地开发用）
type MockTdAPI struct{}

// NewContractQuerier 创建合约查询器（Mock 实现）
func NewContractQuerier() ContractQuerier {
	return &MockTdAPI{}
}

// QueryAllFutures Mock：返回硬编码合约列表
func (m *MockTdAPI) QueryAllFutures(_, _, _, _ string) ([]string, error) {
	log.Printf("[ctp.td.mock] returning %d mock futures symbols", len(mockFutures))
	return mockFutures, nil
}
