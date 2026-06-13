// watchdog/domain — 内存使用率告警（三级）
package domain

const (
	MemWarnLevel1 = 0.80 // 80%：黄色告警
	MemWarnLevel2 = 0.90 // 90%：橙色告警
	MemWarnLevel3 = 0.95 // 95%：红色告警
)

// MemoryAlertLevel 根据内存使用率返回告警级别
func MemoryAlertLevel(usageRatio float64) string {
	switch {
	case usageRatio >= MemWarnLevel3:
		return "critical"
	case usageRatio >= MemWarnLevel2:
		return "warning_high"
	case usageRatio >= MemWarnLevel1:
		return "warning"
	default:
		return "normal"
	}
}
