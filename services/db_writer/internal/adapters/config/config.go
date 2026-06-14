// db_writer/adapters/config -- 配置加载
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config db_writer 服务配置
type Config struct {
	ServiceName     string        // DB_WRITER_SERVICE_NAME, default "db_writer"
	RedisAddr       string        // DB_WRITER_REDIS_ADDR, default "redis:6379"
	InfluxDBURL     string        // DB_WRITER_INFLUXDB_URL
	InfluxDBToken   string        // DB_WRITER_INFLUXDB_TOKEN
	InfluxDBOrg     string        // DB_WRITER_INFLUXDB_ORG, default "arbitrage"
	BatchSize       int           // DB_WRITER_BATCH_SIZE, default 100
	BatchInterval   time.Duration // DB_WRITER_BATCH_INTERVAL, default 1s
	MaxRetries      int           // DB_WRITER_MAX_RETRIES, default 3
	HealthPort      int           // DB_WRITER_HEALTH_PORT, default 8081
	DingtalkWebhook string        // DB_WRITER_DINGTALK_WEBHOOK
	Streams         []string      // 消费的 stream 列表
}

// DefaultStreams 默认消费的 stream 列表
var DefaultStreams = []string{
	"write:tick",
	"write:indicator",
	"write:score",
	"write:order",
	"write:sentiment",
	"write:position_snapshot",
}

// Load 从环境变量加载配置
func Load() Config {
	return Config{
		ServiceName:     getEnv("DB_WRITER_SERVICE_NAME", "db_writer"),
		RedisAddr:       getEnv("DB_WRITER_REDIS_ADDR", "redis:6379"),
		InfluxDBURL:     getEnv("DB_WRITER_INFLUXDB_URL", "http://influxdb:8086"),
		InfluxDBToken:   getEnv("DB_WRITER_INFLUXDB_TOKEN", ""),
		InfluxDBOrg:     getEnv("DB_WRITER_INFLUXDB_ORG", "arbitrage"),
		BatchSize:       getEnvInt("DB_WRITER_BATCH_SIZE", 100),
		BatchInterval:   getEnvDuration("DB_WRITER_BATCH_INTERVAL", 1*time.Second),
		MaxRetries:      getEnvInt("DB_WRITER_MAX_RETRIES", 3),
		HealthPort:      getEnvInt("DB_WRITER_HEALTH_PORT", 8081),
		DingtalkWebhook: getEnv("DB_WRITER_DINGTALK_WEBHOOK", ""),
		Streams:         parseStreams(getEnv("DB_WRITER_STREAMS", "")),
	}
}

// StreamToBucket stream -> InfluxDB bucket 映射表
func StreamToBucket(stream string) string {
	switch stream {
	case "write:tick":
		return "ma_tick"
	case "write:indicator":
		return "indicator"
	case "write:score":
		return "ai_score_history"
	case "write:order":
		return "order_history"
	case "write:sentiment":
		return "raw_intel"
	case "write:position_snapshot":
		return "position_snapshots"
	default:
		return stream
	}
}

// AllBuckets 返回所有已知 bucket 列表（用于初始化指标）
func AllBuckets() []string {
	return []string{
		"ma_tick",
		"indicator",
		"ai_score_history",
		"order_history",
		"raw_intel",
		"position_snapshots",
	}
}

// AllStreams 返回所有已知 stream 列表
func AllStreams() []string {
	return DefaultStreams
}

// parseStreams 解析逗号分隔的 stream 列表
func parseStreams(raw string) []string {
	if raw == "" {
		result := make([]string, len(DefaultStreams))
		copy(result, DefaultStreams)
		return result
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// getEnv 读取环境变量，不存在返回默认值
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt 读取整型环境变量
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// getEnvDuration 读取时间间隔环境变量
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
