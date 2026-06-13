// orchestrator/adapters/audit_writer — 审计日志写入
package audit_writer

// TODO: 实现审计日志写入 Redis Stream write:audit
// 所有修改型操作 XADD write:audit → db_writer 持久化到 InfluxDB
