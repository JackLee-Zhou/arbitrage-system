// orchestrator/domain — 交易日历（甲醇期货 4 时段 + 节假日）
// 注意：本文件零外部依赖，纯函数
package domain

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
	{Start: "21:00", End: "23:00"}, // 夜盘（甲醇有夜盘）
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
	dateStr := now.Format("2006-01-02")
	for _, h := range MarketHolidays2026 {
		if h == dateStr {
			return false
		}
	}
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
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
	// 如果当前在交易时段，返回当前时段结束时间
	timeStr := now.Format("15:04")
	for _, s := range FuturesSessions {
		if timeStr >= s.Start && timeStr <= s.End {
			endTime, _ := time.Parse("15:04", s.End)
			next := time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), 0, 0, now.Location())
			return next
		}
	}
	return now
}
