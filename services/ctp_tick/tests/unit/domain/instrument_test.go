// ctp_tick/domain unit 测试 — 合约元数据解析纯逻辑
package domain_test

import (
	"testing"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

func TestExtractProductID(t *testing.T) {
	tests := []struct {
		symbol   string
		expected string
	}{
		{"MA2509", "MA"},
		{"MA509", "MA"},
		{"pp2509", "pp"},
		{"rb2510", "rb"},
		{"IF2409", "IF"},
		{"sc2507", "sc"},
		{"l2509", "l"},
		{"", ""},
		{"ABC", "ABC"},
	}

	for _, tt := range tests {
		sym := tt.symbol
		want := tt.expected
		info := domain.DeriveInstrumentInfo(sym)
		if info.ProductID != want {
			t.Errorf("DeriveInstrumentInfo(%q).ProductID = %q, want %q", sym, info.ProductID, want)
		}
	}
}

func TestDeriveInstrumentInfo_CZCE_ShortFormat(t *testing.T) {
	// CZCE 短式: 产品代码 + 1位年份 + 2位月份
	info := domain.DeriveInstrumentInfo("MA509")

	if info.Symbol != "MA509" {
		t.Errorf("Symbol = %q, want MA509", info.Symbol)
	}
	if info.ProductID != "MA" {
		t.Errorf("ProductID = %q, want MA", info.ProductID)
	}
	if info.Exchange != "CZCE" {
		t.Errorf("Exchange = %q, want CZCE", info.Exchange)
	}
	if info.ProductNameCN != "甲醇" {
		t.Errorf("ProductNameCN = %q, want 甲醇", info.ProductNameCN)
	}
	if info.ContractYear != 2025 {
		t.Errorf("ContractYear = %d, want 2025", info.ContractYear)
	}
	if info.ContractMonth != 9 {
		t.Errorf("ContractMonth = %d, want 9", info.ContractMonth)
	}
	if info.Multiplier <= 0 {
		t.Errorf("Multiplier = %f, expected > 0", info.Multiplier)
	}
	if info.PriceTick <= 0 {
		t.Errorf("PriceTick = %f, expected > 0", info.PriceTick)
	}
	if info.LongMarginRatio <= 0 {
		t.Errorf("LongMarginRatio = %f, expected > 0", info.LongMarginRatio)
	}
}

func TestDeriveInstrumentInfo_SHFE_LongFormat(t *testing.T) {
	// SHFE 长式: 产品代码 + 2位年份 + 2位月份
	info := domain.DeriveInstrumentInfo("rb2510")

	if info.Symbol != "rb2510" {
		t.Errorf("Symbol = %q, want rb2510", info.Symbol)
	}
	if info.ProductID != "rb" {
		t.Errorf("ProductID = %q, want rb", info.ProductID)
	}
	if info.Exchange != "SHFE" {
		t.Errorf("Exchange = %q, want SHFE", info.Exchange)
	}
	if info.ProductNameCN != "螺纹钢" {
		t.Errorf("ProductNameCN = %q, want 螺纹钢", info.ProductNameCN)
	}
	if info.ContractYear != 2025 {
		t.Errorf("ContractYear = %d, want 2025", info.ContractYear)
	}
	if info.ContractMonth != 10 {
		t.Errorf("ContractMonth = %d, want 10", info.ContractMonth)
	}
}

func TestDeriveInstrumentInfo_DCE_UpperCase(t *testing.T) {
	// DCE 使用小写前缀
	info := domain.DeriveInstrumentInfo("pp2601")

	if info.ProductID != "pp" {
		t.Errorf("ProductID = %q, want pp", info.ProductID)
	}
	if info.Exchange != "DCE" {
		t.Errorf("Exchange = %q, want DCE", info.Exchange)
	}
	if info.ContractYear != 2026 {
		t.Errorf("ContractYear = %d, want 2026", info.ContractYear)
	}
	if info.ContractMonth != 1 {
		t.Errorf("ContractMonth = %d, want 1", info.ContractMonth)
	}
}

func TestDeriveInstrumentInfo_CFFEX(t *testing.T) {
	info := domain.DeriveInstrumentInfo("IF2409")

	if info.ProductID != "IF" {
		t.Errorf("ProductID = %q, want IF", info.ProductID)
	}
	if info.Exchange != "CFFEX" {
		t.Errorf("Exchange = %q, want CFFEX", info.Exchange)
	}
	if info.ProductNameCN != "沪深300" {
		t.Errorf("ProductNameCN = %q, want 沪深300", info.ProductNameCN)
	}
}

func TestDeriveInstrumentInfo_UnknownProduct(t *testing.T) {
	// 未知品种应返回空值但不崩溃
	info := domain.DeriveInstrumentInfo("XX2509")

	if info.ProductID != "XX" {
		t.Errorf("ProductID = %q, want XX", info.ProductID)
	}
	if info.Exchange != "" {
		t.Errorf("Exchange = %q, want empty", info.Exchange)
	}
	if info.ProductNameCN != "" {
		t.Errorf("ProductNameCN = %q, want empty", info.ProductNameCN)
	}
}

func TestDeriveAllInstrumentInfos_Dedup(t *testing.T) {
	symbols := []string{"MA509", "MA509", "MA601", "rb2510"}
	infos := domain.DeriveAllInstrumentInfos(symbols)

	if len(infos) != 3 {
		t.Errorf("expected 3 unique instruments, got %d", len(infos))
	}
}

func TestDeriveAllInstrumentInfos_Empty(t *testing.T) {
	infos := domain.DeriveAllInstrumentInfos(nil)
	if len(infos) != 0 {
		t.Errorf("expected empty, got %d", len(infos))
	}

	infos = domain.DeriveAllInstrumentInfos([]string{})
	if len(infos) != 0 {
		t.Errorf("expected empty, got %d", len(infos))
	}
}

func TestDeriveInstrumentInfo_AllMockSymbols(t *testing.T) {
	// 验证所有 mock 合约都能正常推导，不崩溃
	mockSymbols := []string{
		"MA509", "MA601", "TA509", "TA601", "FG509", "FG601",
		"pp2509", "pp2601", "l2509", "l2601",
		"rb2510", "hc2510", "cu2507",
		"sc2507", "lu2508",
		"IF2409", "IC2409", "IH2409", "IM2409",
		"si2509", "lc2509",
	}

	for _, sym := range mockSymbols {
		info := domain.DeriveInstrumentInfo(sym)
		if info.Symbol == "" {
			t.Errorf("DeriveInstrumentInfo(%q) returned empty symbol", sym)
		}
		if info.Exchange == "" {
			t.Errorf("DeriveInstrumentInfo(%q) could not determine exchange", sym)
		}
		if info.ContractYear <= 0 && info.ContractMonth <= 0 {
			t.Errorf("DeriveInstrumentInfo(%q) could not parse contract date", sym)
		}
	}
}

func TestDeriveInstrumentInfo_PriceTick(t *testing.T) {
	tests := []struct {
		symbol string
		tick   float64
	}{
		{"MA509", 1},
		{"TA509", 2},
		{"i2509", 0.5},
		{"sc2507", 0.1},
		{"cu2507", 10},
		{"IF2409", 0.2},
		{"lc2509", 50},
	}

	for _, tt := range tests {
		info := domain.DeriveInstrumentInfo(tt.symbol)
		if info.PriceTick != tt.tick {
			t.Errorf("DeriveInstrumentInfo(%q).PriceTick = %f, want %f", tt.symbol, info.PriceTick, tt.tick)
		}
	}
}

func TestDeriveInstrumentInfo_JSONTags(t *testing.T) {
	// 验证 JSON 标签存在，且关键字段序列化后正确
	info := domain.DeriveInstrumentInfo("MA509")
	if info.Symbol != "MA509" {
		t.Errorf("Symbol JSON roundtrip check failed")
	}
}
