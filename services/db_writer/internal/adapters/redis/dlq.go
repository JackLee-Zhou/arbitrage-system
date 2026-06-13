// db_writer/adapters/redis — 死信队列操作
package redis

// TODO: 实现死信队列（DLQ）的 Redis 操作
// 写入死信：XADD dlq:write:<bucket> <data>
// 读取死信：XREAD dlq:write:<bucket>
// 清理死信：EXPIRE dlq:write:<bucket> 604800（7 天）
