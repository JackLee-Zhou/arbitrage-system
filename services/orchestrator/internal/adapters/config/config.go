// orchestrator/adapters/config — 环境变量配置加载
package config

import (
	"os"
	"strings"
	"time"
)

// Config orchestrator 服务配置
type Config struct {
	ServiceName      string // ORCHESTRATOR_SERVICE_NAME
	ServiceHost      string // ORCHESTRATOR_SERVICE_HOST（空→容器名）
	HTTPPort         string // HTTP_PORT
	RedisAddr        string // REDIS_ADDR
	TradingAgentGRPC string // TRADING_AGENT_GRPC
	InfluxURL        string // INFLUXDB_URL
	InfluxToken      string // INFLUXDB_TOKEN
	InfluxOrg        string // INFLUXDB_ORG
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	ShutdownTimeout  time.Duration
}

// Load 从环境变量加载配置
func Load() Config {
	return Config{
		ServiceName:      getEnv("ORCHESTRATOR_SERVICE_NAME", "orchestrator"),
		ServiceHost:      getEnv("ORCHESTRATOR_SERVICE_HOST", ""),
		HTTPPort:         getEnv("HTTP_PORT", "8080"),
		RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
		TradingAgentGRPC: getEnv("TRADING_AGENT_GRPC", "trading_agent:9090"),
		InfluxURL:        getEnv("INFLUXDB_URL", "http://influxdb:8086"),
		InfluxToken:      getEnv("INFLUXDB_TOKEN", ""),
		InfluxOrg:        getEnv("INFLUXDB_ORG", "arbitrage"),
		ReadTimeout:      getEnvDuration("HTTP_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:     getEnvDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:      getEnvDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
		ShutdownTimeout:  getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// SplitTrim 按分隔符切分字符串并去除空白
func SplitTrim(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
