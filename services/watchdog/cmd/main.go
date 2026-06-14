// watchdog — 健康监控入口
// 从 Redis 自动发现需要监控的服务（fallback 到 HEALTHZ_TARGETS 环境变量）
// 服务注册时已写入三元组（host:port + name），watchdog 直接读取使用
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
	pkgregistry "github.com/arbitrage-system/pkg/registry"

	"github.com/arbitrage-system/watchdog/internal/adapters"
	"github.com/arbitrage-system/watchdog/internal/domain"
)

// serviceVersion 当前服务版本
const serviceVersion = "0.1.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[watchdog] starting...")

	cfg := loadConfig()

	// ---- 服务注册 ----
	redisClient := pkgredis.NewAdapter(cfg.RedisAddr)
	registryCancel, err := pkgregistry.RegisterAndKeepAlive(
		context.Background(), redisClient, pkgregistry.ServiceInfo{
			Name:         "watchdog",
			Version:      serviceVersion,
			StartedAt:    time.Now(),
			HealthURL:    ":8081",
			Capabilities: []string{"health_check", "disk_monitor", "memory_monitor"},
		},
	)
	if err != nil {
		log.Printf("[watchdog] warning: service registration failed: %v (continuing)", err)
	}
	defer func() {
		if registryCancel != nil {
			registryCancel()
		}
	}()

	go healthCheckLoop(cfg)
	go diskMonitorLoop(cfg)
	go memoryMonitorLoop(cfg)

	log.Println("[watchdog] all monitors started")

	// ---- 等待退出信号 ----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[watchdog] received signal %v, shutting down...", sig)
}

type Config struct {
	RedisAddr      string
	HealthzTargets []string
	ScrapeInterval time.Duration
	DingtalkURL    string
	ProbeTimeout   time.Duration
}

func loadConfig() Config {
	targetsStr := getEnv("HEALTHZ_TARGETS", "")
	targets := strings.Split(targetsStr, ",")
	trimmed := make([]string, 0, len(targets))
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t != "" {
			trimmed = append(trimmed, t)
		}
	}

	scrapeInterval := 5 * time.Second
	if v := os.Getenv("SCRAPE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			scrapeInterval = d
		}
	}

	probeTimeout := 3 * time.Second
	if v := os.Getenv("PROBE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			probeTimeout = d
		}
	}

	return Config{
		RedisAddr:      getEnv("REDIS_ADDR", "redis:6379"),
		HealthzTargets: trimmed,
		ScrapeInterval: scrapeInterval,
		DingtalkURL:    getEnv("DINGTALK_WEBHOOK_URL", ""),
		ProbeTimeout:   probeTimeout,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ============================================================
// 服务发现：从 Redis 读取注册信息（三元组：host:port + name）
// ============================================================

type discoveredTarget struct {
	Name     string // 服务名，如 "ctp_tick"（日志显示）
	HostPort string // 探测地址，如 "localhost:8081"（直接来自 health_url）
}

// serviceRegistration 只解析需要的字段
type serviceRegistration struct {
	Name      string `json:"name"`
	HealthURL string `json:"health_url"`
}

// discoverFromRedis 从 Redis services:* 读取已注册的服务
// health_url 已是完整的 host:port，直接当探活地址用
func discoverFromRedis(redisClient pkgredis.RedisClient) ([]discoveredTarget, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	keys, err := redisClient.Keys(ctx, "services:*")
	if err != nil {
		return nil, fmt.Errorf("redis KEYS services:* failed: %w", err)
	}
	if len(keys) == 0 {
		return nil, nil
	}

	var targets []discoveredTarget
	for _, key := range keys {
		raw, err := redisClient.Get(ctx, key)
		if err != nil || raw == "" {
			continue
		}
		var info serviceRegistration
		if err := json.Unmarshal([]byte(raw), &info); err != nil || info.Name == "" || info.HealthURL == "" {
			continue
		}
		targets = append(targets, discoveredTarget{
			Name:     info.Name,
			HostPort: info.HealthURL,
		})
	}

	return targets, nil
}

// ============================================================
// 健康检查循环
// ============================================================

func healthCheckLoop(cfg Config) {
	log.Printf("[watchdog] redis=%s interval=%s fallback=%v",
		cfg.RedisAddr, cfg.ScrapeInterval, cfg.HealthzTargets)

	client := adapters.NewHealthzClient(cfg.ProbeTimeout)
	dingtalk := adapters.NewDingTalkAlert(cfg.DingtalkURL, 5*time.Second)
	ticker := time.NewTicker(cfg.ScrapeInterval)
	defer ticker.Stop()

	// Redis 客户端，连接失败则降级为 HEALTHZ_TARGETS
	redisClient := pkgredis.NewAdapter(cfg.RedisAddr)
	if err := redisClient.Ping(context.Background()); err != nil {
		log.Printf("[watchdog] Redis unreachable (%v), fallback to HEALTHZ_TARGETS", err)
		redisClient = nil
	}

	lastStatus := make(map[string]string)
	serviceNames := make(map[string]string)
	var targetsMu sync.RWMutex
	currentTargets := make(map[string]string) // hostPort → name

	// 每 30s 刷新一次服务发现
	discoverTicker := time.NewTicker(30 * time.Second)
	defer discoverTicker.Stop()

	refreshTargets(redisClient, &targetsMu, currentTargets, cfg)

	for {
		select {
		case <-discoverTicker.C:
			refreshTargets(redisClient, &targetsMu, currentTargets, cfg)

		case <-ticker.C:
			targetsMu.RLock()
			pairs := make([]struct{ hostPort, name string }, 0, len(currentTargets))
			for hp, n := range currentTargets {
				pairs = append(pairs, struct{ hostPort, name string }{hp, n})
			}
			targetsMu.RUnlock()

			for _, p := range pairs {
				status := client.Probe(p.hostPort)

				// 优先用 /healthz 响应里的 service 字段
				name := status.Service
				if name == "" {
					if cached, ok := serviceNames[p.hostPort]; ok {
						name = cached
					} else {
						name = p.name
					}
				} else {
					serviceNames[p.hostPort] = name
				}

				prev, existed := lastStatus[p.hostPort]
				lastStatus[p.hostPort] = status.Status

				switch status.Status {
				case "unhealthy":
					log.Printf("[watchdog] 🔴 %s UNHEALTHY (%s, latency=%s)", name, p.hostPort, formatLatency(status.Latency))
					if !existed || prev != "unhealthy" {
						dingtalk.Send("服务异常: "+name,
							name+" 不可达\n- 地址: "+p.hostPort+"\n- 延迟: "+formatLatency(status.Latency))
					}
				case "degraded":
					log.Printf("[watchdog] 🟡 %s DEGRADED (%s, latency=%s)", name, p.hostPort, formatLatency(status.Latency))
					if !existed || prev == "healthy" {
						dingtalk.Send("服务降级: "+name,
							name+" 降级\n- 地址: "+p.hostPort+"\n- 延迟: "+formatLatency(status.Latency))
					}
				default:
					log.Printf("[watchdog] 🟢 %s healthy (%s, latency=%s)", name, p.hostPort, formatLatency(status.Latency))
					if existed && prev != "healthy" {
						dingtalk.Send("服务恢复: "+name, name+" 已恢复")
					}
				}

				if domain.IsUnhealthy(status) {
					log.Printf("[watchdog] ALERT: %s needs attention!", name)
				}
			}
		}
	}
}

// refreshTargets 从 Redis 或 fallback 刷新目标列表
func refreshTargets(
	redisClient pkgredis.RedisClient,
	mu *sync.RWMutex,
	current map[string]string,
	cfg Config,
) {
	var newTargets map[string]string

	if redisClient != nil {
		discovered, err := discoverFromRedis(redisClient)
		if err != nil {
			log.Printf("[watchdog] discovery error: %v", err)
		}
		if len(discovered) > 0 {
			newTargets = make(map[string]string, len(discovered))
			for _, t := range discovered {
				newTargets[t.HostPort] = t.Name
			}
		}
	}

	if len(newTargets) == 0 && len(cfg.HealthzTargets) > 0 {
		newTargets = make(map[string]string, len(cfg.HealthzTargets))
		for _, t := range cfg.HealthzTargets {
			newTargets[t] = t
		}
	}

	if len(newTargets) == 0 {
		return
	}

	mu.Lock()
	for k := range current {
		delete(current, k)
	}
	for k, v := range newTargets {
		current[k] = v
	}
	mu.Unlock()

	names := make([]string, 0, len(newTargets))
	for _, n := range newTargets {
		names = append(names, n)
	}
	log.Printf("[watchdog] targets refreshed: %v", names)
}

// ============================================================
// 磁盘 / 内存监控
// ============================================================

func diskMonitorLoop(cfg Config) {
	log.Printf("[watchdog] disk monitor started")
	dingtalk := adapters.NewDingTalkAlert(cfg.DingtalkURL, 5*time.Second)
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		usage := checkDiskUsage()
		if usage >= 0 {
			log.Printf("[watchdog] disk: %.1f%%", usage)
			if usage > 95 {
				dingtalk.Send("磁盘告警 [严重]", fmt.Sprintf("使用率 %.1f%%, 请立即清理", usage))
			} else if usage > 90 {
				dingtalk.Send("磁盘告警 [高危]", fmt.Sprintf("使用率 %.1f%%, 建议扩容", usage))
			} else if usage > 60 {
				dingtalk.Send("磁盘告警 [提醒]", fmt.Sprintf("使用率 %.1f%%, 建议关注", usage))
			}
		}
	}
}

func memoryMonitorLoop(cfg Config) {
	log.Printf("[watchdog] memory monitor started")
	dingtalk := adapters.NewDingTalkAlert(cfg.DingtalkURL, 5*time.Second)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	lastAlerted := make(map[string]time.Time)

	for range ticker.C {
		usage, level := checkMemoryUsage()
		if usage >= 0 {
			log.Printf("[watchdog] memory: %.1f%% (%s)", usage, level)
			if level != "" {
				if last, ok := lastAlerted[level]; !ok || time.Since(last) > time.Hour {
					lastAlerted[level] = time.Now()
					dingtalk.Send("内存告警: "+level, fmt.Sprintf("使用率 %.1f%%", usage))
				}
			}
		}
	}
}

// ============================================================
// 辅助函数
// ============================================================

func formatLatency(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func checkDiskUsage() float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		log.Printf("[watchdog] statfs error: %v", err)
		return -1
	}
	return float64(stat.Blocks-stat.Bavail) / float64(stat.Blocks) * 100
}

func checkMemoryUsage() (float64, string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	usage := float64(m.Alloc) / float64(m.Sys) * 100
	switch {
	case usage > 95:
		return usage, "critical"
	case usage > 90:
		return usage, "warning_high"
	case usage > 80:
		return usage, "warning"
	default:
		return usage, ""
	}
}
