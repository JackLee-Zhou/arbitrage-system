// ctp_tick/domain unit 测试 — 最新价快照覆盖式更新
package domain_test

import (
	"testing"
	"time"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

func TestLastPriceStore_UpdateAndGet(t *testing.T) {
	store := domain.NewLastPriceStore()
	if store.Len() != 0 {
		t.Fatalf("expected empty store, got len=%d", store.Len())
	}

	tick := &domain.StandardTick{
		Symbol:       "MA2509",
		Timestamp:    time.Now(),
		LastPrice:    2350.0,
		Volume:       100000,
		OpenPrice:    2345.0,
		HighPrice:    2355.0,
		LowPrice:     2340.0,
		PreClose:     2346.0,
		OpenInterest: 500000,
	}

	store.Update(tick)

	if store.Len() != 1 {
		t.Fatalf("expected len=1, got %d", store.Len())
	}

	snap, ok := store.Get("MA2509")
	if !ok {
		t.Fatal("expected to find MA2509")
	}
	if snap.LastPrice != 2350.0 {
		t.Errorf("last_price = %f, want 2350.0", snap.LastPrice)
	}
	if snap.Open != 2345.0 {
		t.Errorf("open = %f, want 2345.0", snap.Open)
	}
	if snap.High != 2355.0 {
		t.Errorf("high = %f, want 2355.0", snap.High)
	}
	if snap.Low != 2340.0 {
		t.Errorf("low = %f, want 2340.0", snap.Low)
	}
	if snap.PreClose != 2346.0 {
		t.Errorf("pre_close = %f, want 2346.0", snap.PreClose)
	}
	if snap.Volume != 100000 {
		t.Errorf("volume = %d, want 100000", snap.Volume)
	}
	if snap.OpenInterest != 500000 {
		t.Errorf("open_interest = %f, want 500000", snap.OpenInterest)
	}
}

func TestLastPriceStore_ChangePct(t *testing.T) {
	store := domain.NewLastPriceStore()

	tests := []struct {
		name      string
		lastPrice float64
		preClose  float64
		wantPct   float64
	}{
		{"price up", 2400.0, 2350.0, 2.127659574468085},    // (2400-2350)/2350*100
		{"price down", 2300.0, 2350.0, -2.127659574468085}, // (2300-2350)/2350*100
		{"no change", 2350.0, 2350.0, 0},
		{"zero pre_close", 2350.0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tick := &domain.StandardTick{
				Symbol:    "TEST",
				LastPrice: tt.lastPrice,
				PreClose:  tt.preClose,
			}
			store.Update(tick)
			snap, ok := store.Get("TEST")
			if !ok {
				t.Fatal("expected to find TEST")
			}
			if snap.ChangePct != tt.wantPct {
				t.Errorf("change_pct = %f, want %f", snap.ChangePct, tt.wantPct)
			}
		})
	}
}

func TestLastPriceStore_OverwriteKeepsHighLow(t *testing.T) {
	store := domain.NewLastPriceStore()

	// 第一次更新：价格 2350
	store.Update(&domain.StandardTick{
		Symbol:    "MA2509",
		LastPrice: 2350.0,
		HighPrice: 2360.0,
		LowPrice:  2340.0,
		PreClose:  2340.0,
	})

	// 第二次更新：价格 2345，High 低于上次 High 应保留，Low 高于上次 Low 应保留
	store.Update(&domain.StandardTick{
		Symbol:    "MA2509",
		LastPrice: 2345.0,
		HighPrice: 2350.0, // 新 High 低于之前的 2360
		LowPrice:  2345.0, // 新 Low 高于之前的 2340
		PreClose:  2340.0,
	})

	snap, ok := store.Get("MA2509")
	if !ok {
		t.Fatal("expected to find MA2509")
	}
	if snap.High != 2360.0 {
		t.Errorf("high should retain max=2360, got %f", snap.High)
	}
	if snap.Low != 2340.0 {
		t.Errorf("low should retain min=2340, got %f", snap.Low)
	}
}

func TestLastPriceStore_MultipleSymbols(t *testing.T) {
	store := domain.NewLastPriceStore()

	symbols := []string{"MA2509", "RB2510", "SC2601"}
	for i, sym := range symbols {
		store.Update(&domain.StandardTick{
			Symbol:    sym,
			LastPrice: 2350.0 + float64(i)*100,
			PreClose:  2340.0,
		})
	}

	if store.Len() != 3 {
		t.Fatalf("expected len=3, got %d", store.Len())
	}

	all := store.Snapshot()
	if len(all) != 3 {
		t.Fatalf("expected snapshot len=3, got %d", len(all))
	}

	for _, sym := range symbols {
		if _, ok := all[sym]; !ok {
			t.Errorf("snapshot missing symbol %s", sym)
		}
	}
}

func TestLastPriceStore_GetNonexistent(t *testing.T) {
	store := domain.NewLastPriceStore()
	_, ok := store.Get("NONEXIST")
	if ok {
		t.Error("expected ok=false for nonexistent symbol")
	}
}

func TestLastPriceStore_SnapshotCopy(t *testing.T) {
	store := domain.NewLastPriceStore()

	store.Update(&domain.StandardTick{
		Symbol:    "MA2509",
		LastPrice: 2350.0,
		PreClose:  2340.0,
	})

	// 获取副本并修改它
	all := store.Snapshot()
	all["MA2509"].LastPrice = 9999.0

	// 确认原始数据未变
	snap, _ := store.Get("MA2509")
	if snap.LastPrice == 9999.0 {
		t.Error("Snapshot() should return a copy, original was modified")
	}
}

func TestLastPriceStore_ExtremeChangePct(t *testing.T) {
	store := domain.NewLastPriceStore()

	// 极端涨跌幅（超过 MaxChangePct=100）
	tick := &domain.StandardTick{
		Symbol:    "TEST",
		LastPrice: 50000.0,
		PreClose:  100.0,
	}

	store.Update(tick)
	snap, ok := store.Get("TEST")
	if !ok {
		t.Fatal("expected to find TEST")
	}
	// changePct = (50000-100)/100*100 = 49900, 超过限制，应返回 0
	if snap.ChangePct != 0 {
		t.Errorf("change_pct should be clamped to 0, got %f", snap.ChangePct)
	}
}
