// db_writer/domain — 死信队列处理逻辑
package domain

// DLQEntry 死信条目
type DLQEntry struct {
	Bucket     string // 原始目标 bucket
	Data       []byte // 原始数据
	RetryCount int    // 已重试次数
	LastError  string // 最后一次错误信息
}

// DLQAlertThreshold 死信队列告警阈值（条数 > 10 推钉钉）
const DLQAlertThreshold = 10

// ShouldAlert 判断是否应该触发死信告警
func ShouldAlert(dlqSize int) bool {
	return dlqSize >= DLQAlertThreshold
}

// DLQTTL 死信数据保留时间（7 天）
const DLQTTL = 168 * 3600 // 7 天 = 168 小时 = 604800 秒
