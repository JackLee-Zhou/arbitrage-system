// orchestrator/domain/calendar — 中国期货交易日历
package calendar

import "time"

// TradingSession 交易时段
type TradingSession struct {
	Start string // "09:00"
	End   string // "10:15"
}

// 甲醇期货 MA 主力合约交易时段
var FuturesSessions = []TradingSession{
	{Start: "09:00", End: "10:15"}, // 上午前盘
	{Start: "10:30", End: "11:30"}, // 上午后盘
	{Start: "13:30", End: "15:00"}, // 下午盘
	{Start: "21:00", End: "23:00"}, // 夜盘
}

// 节假日清单（2026 年；每年初手动更新）
var MarketHolidays2026 = []string{
	"2026-01-01",                                                         // 元旦
	"2026-02-16", "2026-02-17", "2026-02-18", "2026-02-19", "2026-02-20", // 春节
	"2026-04-06",                             // 清明
	"2026-05-01", "2026-05-04", "2026-05-05", // 劳动节
	"2026-05-31",                                                         // 端午
	"2026-09-25",                                                         // 中秋
	"2026-10-01", "2026-10-02", "2026-10-05", "2026-10-06", "2026-10-07", // 国庆
}

// IsInTradingSession 判断当前时间是否在交易时段
func IsInTradingSession(now time.Time) bool {
	// 先检查是否是节假日
	dateStr := now.Format("2006-01-02")
	for _, h := range MarketHolidays2026 {
		if h == dateStr {
			return false
		}
	}
	// 检查是否是周末
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	// 检查是否在交易时段内
	timeStr := now.Format("15:04")
	for _, s := range FuturesSessions {
		if timeStr >= s.Start && timeStr <= s.End {
			return true
		}
	}
	return false
}

// NextNonTradingWindow 返回下一个非交易时段的开始时间
func NextNonTradingWindow(now time.Time) time.Time {
	// TODO: 精确计算下一个可操作时间窗口
	return now.Add(1 * time.Hour)
}
