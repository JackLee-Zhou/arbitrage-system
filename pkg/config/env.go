// pkg/config — 共享环境变量读取工具
// 消除 5 个服务中重复的 getEnv() 实现
package config

import (
	"os"
	"strconv"
	"time"
)

// GetEnv 读取环境变量，未设置时返回 fallback。
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GetEnvInt 读取整型环境变量，解析失败时返回 fallback。
func GetEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// GetEnvDuration 读取 duration 型环境变量（如 "10s", "1m"），
// 解析失败时返回 fallback。
func GetEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
