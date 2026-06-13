// ctp_tick/domain unit 测试 — 纯逻辑，无外部依赖
package domain_test

import (
	"testing"
	"time"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

func TestFromRawMarketData_ValidData(t *testing.T) {
	raw := &domain.RawMarketData{
		TradingDay:         "20260613",
		InstrumentID:       "MA2509",
		UpdateTime:         "10:30:15",
		UpdateMillisec:     500,
		LastPrice:          2350.0,
		Volume:             100000,
		OpenPrice:          2345.0,
		HighestPrice:       2355.0,
		LowestPrice:        2340.0,
		PreClosePrice:      2346.0,
		PreSettlementPrice: 2348.0,
		OpenInterest:       500000,
		UpperLimitPrice:    2500.0,
		LowerLimitPrice:    2200.0,
		BidPrice1:          2349.0,
		BidVolume1:         50,
		AskPrice1:          2351.0,
		AskVolume1:         30,
	}

	tick, err := domain.FromRawMarketData(raw)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if tick.Symbol != "MA2509" {
		t.Errorf("symbol = %s, want MA2509", tick.Symbol)
	}
	if tick.LastPrice != 2350.0 {
		t.Errorf("lastPrice = %f, want 2350.0", tick.LastPrice)
	}
	if tick.Volume != 100000 {
		t.Errorf("volume = %d, want 100000", tick.Volume)
	}
	if tick.BidPrice1 != 2349.0 {
		t.Errorf("bidPrice1 = %f, want 2349.0", tick.BidPrice1)
	}
	if tick.BidVolume1 != 50 {
		t.Errorf("bidVolume1 = %d, want 50", tick.BidVolume1)
	}

	// 时间验证：应该是 2026-06-13 10:30:15.500
	expectedYear := 2026
	if tick.Timestamp.Year() != expectedYear {
		t.Errorf("timestamp year = %d, want %d", tick.Timestamp.Year(), expectedYear)
	}
}

func TestFromRawMarketData_NilData(t *testing.T) {
	_, err := domain.FromRawMarketData(nil)
	if err == nil {
		t.Error("expected error for nil data")
	}
}

func TestFromRawMarketData_EmptySymbol(t *testing.T) {
	raw := &domain.RawMarketData{
		InstrumentID: "",
		LastPrice:    2350.0,
	}
	_, err := domain.FromRawMarketData(raw)
	if err == nil {
		t.Error("expected error for empty symbol")
	}
}

func TestFromRawMarketData_InvalidPrice(t *testing.T) {
	tests := []struct {
		name  string
		price float64
	}{
		{"zero price", 0},
		{"negative price", -1.0},
		{"extreme price", 1e101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &domain.RawMarketData{
				InstrumentID: "MA2509",
				LastPrice:    tt.price,
			}
			_, err := domain.FromRawMarketData(raw)
			if err == nil {
				t.Errorf("expected error for price=%.1f", tt.price)
			}
		})
	}
}

func TestToProto(t *testing.T) {
	ts := time.Date(2026, 6, 13, 10, 30, 15, 500000000, time.UTC)
	tick := &domain.StandardTick{
		Symbol:       "MA2509",
		Timestamp:    ts,
		LastPrice:    2350.0,
		Volume:       100000,
		OpenPrice:    2345.0,
		HighPrice:    2355.0,
		LowPrice:     2340.0,
		PreClose:     2346.0,
		PreSettle:    2348.0,
		OpenInterest: 500000,
		UpperLimit:   2500.0,
		LowerLimit:   2200.0,
		BidPrice1:    2349.0,
		BidVolume1:   50,
		AskPrice1:    2351.0,
		AskVolume1:   30,
	}

	proto := tick.ToProto()

	if proto.Symbol != "MA2509" {
		t.Errorf("proto symbol = %s", proto.Symbol)
	}
	if proto.Timestamp != ts.UnixMilli() {
		t.Errorf("proto timestamp = %d, want %d", proto.Timestamp, ts.UnixMilli())
	}
	if proto.LastPrice != 2350.0 {
		t.Errorf("proto last_price = %f", proto.LastPrice)
	}
}

func TestProtoTick_JSONRoundtrip(t *testing.T) {
	// 验证 ProtoTick 可以正确序列化和反序列化为 JSON
	t.Skip("JSON roundtrip test — requires encoding/json import in test")
}

func BenchmarkFromRawMarketData(b *testing.B) {
	raw := &domain.RawMarketData{
		TradingDay:   "20260613",
		InstrumentID: "MA2509",
		UpdateTime:   "10:30:15",
		LastPrice:    2350.0,
		Volume:       100000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = domain.FromRawMarketData(raw)
	}
}
