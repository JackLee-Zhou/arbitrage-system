// orchestrator/adapters — 服务健康巡检器
// 从 Redis 发现已注册的服务，周期性探测 /healthz，缓存结果供 REST API 暴露
package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// ServiceHealth 单个服务的健康快照
type ServiceHealth struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // healthy / degraded / unhealthy
	Latency int64  `json:"latency_ms"`
	Address string `json:"address"` // host:port
	Error   string `json:"error,omitempty"`
}

// serviceRegistration 服务注册 JSON 格式
type serviceReg struct {
	Name      string `json:"name"`
	HealthURL string `json:"health_url"`
}

// HealthPoller 服务健康巡检器
type HealthPoller struct {
	redisClient pkgredis.RedisClient
	interval    time.Duration
	probeClient *http.Client

	mu       sync.RWMutex
	services map[string]ServiceHealth // key: name
}

// NewHealthPoller 创建健康巡检器
func NewHealthPoller(redisClient pkgredis.RedisClient, interval time.Duration) *HealthPoller {
	return &HealthPoller{
		redisClient: redisClient,
		interval:    interval,
		probeClient: &http.Client{Timeout: 3 * time.Second},
		services:    make(map[string]ServiceHealth),
	}
}

// Start 启动后台巡检循环，返回停止函数
func (p *HealthPoller) Start(ctx context.Context) {
	p.refresh(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.refresh(ctx)
		}
	}
}

// Snapshot 返回当前所有服务的健康快照
func (p *HealthPoller) Snapshot() []ServiceHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()
	snapshot := make([]ServiceHealth, 0, len(p.services))
	for _, s := range p.services {
		snapshot = append(snapshot, s)
	}
	return snapshot
}

// refresh 执行一次全量巡检
func (p *HealthPoller) refresh(ctx context.Context) {
	discovered := p.discover(ctx)

	var wg sync.WaitGroup
	results := make(map[string]ServiceHealth, len(discovered))
	var mu sync.Mutex

	for _, svc := range discovered {
		wg.Add(1)
		go func(s ServiceHealth) {
			defer wg.Done()
			probed := p.probe(s)
			mu.Lock()
			results[s.Name] = probed
			mu.Unlock()
		}(svc)
	}
	wg.Wait()

	p.mu.Lock()
	// 即使 discovered 为空也更新（清空旧缓存，反映没有在线服务）
	p.services = results
	p.mu.Unlock()
}

// discover 从 Redis 获取已注册的服务列表
func (p *HealthPoller) discover(ctx context.Context) []ServiceHealth {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	keys, err := p.redisClient.Keys(ctx, "services:*")
	if err != nil {
		return nil
	}

	var services []ServiceHealth
	for _, key := range keys {
		raw, err := p.redisClient.Get(ctx, key)
		if err != nil || raw == "" {
			continue
		}
		var reg serviceReg
		if err := json.Unmarshal([]byte(raw), &reg); err != nil || reg.Name == "" || reg.HealthURL == "" {
			continue
		}
		services = append(services, ServiceHealth{
			Name:    reg.Name,
			Address: reg.HealthURL,
		})
	}
	return services
}

// probe 探测单个服务的 /healthz
func (p *HealthPoller) probe(svc ServiceHealth) ServiceHealth {
	url := "http://" + svc.Address + "/healthz"
	start := time.Now()

	resp, err := p.probeClient.Get(url)
	latency := time.Since(start).Milliseconds()
	svc.Latency = latency

	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		svc.Status = "unhealthy"
		svc.Error = err.Error()
		return svc
	}

	var result struct {
		Status string `json:"status"`
		Checks map[string]struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if resp.StatusCode == http.StatusOK {
			svc.Status = "healthy"
		} else {
			svc.Status = "unhealthy"
		}
		return svc
	}

	svc.Status = result.Status
	return svc
}
