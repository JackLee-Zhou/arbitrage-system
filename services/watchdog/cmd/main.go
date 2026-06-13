// watchdog — 健康监控入口
package main

import (
	"log"
	"os"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[watchdog] starting...")

	cfg := loadConfig()

	// 启动健康巡检
	go healthCheckLoop(cfg)

	// 启动磁盘监控
	go diskMonitorLoop(cfg)

	// 启动内存监控
	go memoryMonitorLoop(cfg)

	log.Println("[watchdog] all monitors started")

	// 阻塞
	select {}
}

type Config struct {
	HealthzTargets string
	ScrapeInterval time.Duration
	DingtalkURL    string
}

func loadConfig() Config {
	return Config{
		HealthzTargets: getEnv("HEALTHZ_TARGETS", "ctp_tick:8081,trading_agent:8081"),
		ScrapeInterval: 5 * time.Second,
		DingtalkURL:    getEnv("DINGTALK_WEBHOOK_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func healthCheckLoop(cfg Config) {
	// TODO: 周期性 HTTP GET 各服务的 /healthz
	// 任意一个不健康 → 推钉钉
	log.Println("[watchdog] health check loop started")
}

func diskMonitorLoop(cfg Config) {
	// TODO: 监控系统盘使用率
	// >60% 推钉钉告警（提醒购买数据盘）
	log.Println("[watchdog] disk monitor loop started")
}

func memoryMonitorLoop(cfg Config) {
	// TODO: 三级内存告警
	// 80%（黄色）/ 90%（橙色）/ 95%（红色）
	// 每个级别 1 小时内只推一次（去重）
	log.Println("[watchdog] memory monitor loop started")
}
