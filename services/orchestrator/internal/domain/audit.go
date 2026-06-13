// orchestrator/domain — 审计日志结构定义（纯逻辑）
package domain

import "time"

// AuditEntry 操作审计条目
type AuditEntry struct {
	ID        string    // uuid
	Timestamp time.Time // 操作时间
	Operator  string    // 操作人（"dashboard_user" / "system"）
	Action    string    // "emergency_close" / "pause_strategy" / "update_risk" / "promote_shadow"
	Target    string    // 操作对象（策略名 / 风控参数名）
	OldValue  string    // 变更前值
	NewValue  string    // 变更后值
	Result    string    // "success" / "failed"
	Error     string    // 失败原因
	SourceIP  string    // 请求来源 IP
}

// IsMutation 判断操作是否为修改型操作（需要走交易时段守卫）
func IsMutation(audit *AuditEntry) bool {
	switch audit.Action {
	case "emergency_close", "pause_strategy", "resume_strategy",
		"update_risk", "promote_shadow", "create_strategy",
		"edit_strategy", "delete_strategy":
		return true
	}
	return false
}
