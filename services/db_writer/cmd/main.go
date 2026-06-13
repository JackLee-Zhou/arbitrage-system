// db_writer — InfluxDB 串行写入服务入口
package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[db_writer] starting...")

	cfg := loadConfig()

	// 连接 Redis
	log.Printf("[db_writer] connecting to Redis at %s...", cfg.RedisAddr)

	// 连接 InfluxDB
	log.Printf("[db_writer] connecting to InfluxDB at %s...", cfg.InfluxDBURL)

	// 启动消费组
	go consumeWriteStream("write:tick")
	go consumeWriteStream("write:indicator")
	go consumeWriteStream("write:score")
	go consumeWriteStream("write:order")
	go consumeWriteStream("write:sentiment")
	go consumeWriteStream("write:position_snapshot")

	// 启动死信队列监控
	go monitorDLQ()

	// 启动 /healthz + /metrics
	startHealthServer(cfg.HealthPort)

	log.Println("[db_writer] all consumers started")

	// 阻塞
	select {}
}

type Config struct {
	RedisAddr   string
	InfluxDBURL string
	HealthPort  string
}

func loadConfig() Config {
	return Config{
		RedisAddr:   getEnv("REDIS_ADDR", "redis:6379"),
		InfluxDBURL: getEnv("INFLUXDB_URL", "http://influxdb:8086"),
		HealthPort:  getEnv("HEALTH_PORT", "8081"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func consumeWriteStream(stream string) {
	log.Printf("[db_writer] consuming stream: %s", stream)
	// TODO: 实现 XREADGROUP 消费 → 批量聚合 → InfluxDB 写入 → XACK
}

func monitorDLQ() {
	// TODO: 监控死信队列 dlq:write:*，有数据 → 推钉钉告警
}

func startHealthServer(port string) {
	// TODO: 实现 /healthz + /metrics
	log.Printf("[db_writer] health server on :%s", port)
}
