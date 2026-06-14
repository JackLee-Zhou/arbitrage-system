// watchdog/adapters — 健康检查 HTTP 客户端
package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/arbitrage-system/watchdog/internal/domain"
)

// HealthzClient HTTP 健康检查客户端
type HealthzClient struct {
	client *http.Client
}

// NewHealthzClient 创建健康检查客户端
func NewHealthzClient(timeout time.Duration) *HealthzClient {
	return &HealthzClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Probe 探测单个目标的 /healthz 端点，失败重试 1 次
// target 格式: "host:port"（如 "ctp_tick:8081"）
func (h *HealthzClient) Probe(target string) domain.HealthStatus {
	url := fmt.Sprintf("http://%s/healthz", target)

	status := h.doProbe(url)
	if status.Status == "unhealthy" {
		// 失败重试 1 次
		log.Printf("[healthz_client] retrying %s after failure: %s", target, status.Status)
		time.Sleep(500 * time.Millisecond)
		status = h.doProbe(url)
	}

	// 不做 fallback：调用方（healthCheckLoop）会用自己的 serviceNames 缓存处理空名
	return status
}

// doProbe 执行单次 HTTP GET 探测
func (h *HealthzClient) doProbe(url string) domain.HealthStatus {
	start := time.Now()

	resp, err := h.client.Get(url)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return domain.HealthStatus{
			Status:  "unhealthy",
			Latency: latency,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// 尝试解析 JSON 响应
	var result domain.HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// JSON 解析失败，根据 HTTP 状态码判断
		if resp.StatusCode == http.StatusOK {
			return domain.HealthStatus{
				Status:  "healthy",
				Latency: latency,
			}
		}
		return domain.HealthStatus{
			Status:  "unhealthy",
			Latency: latency,
		}
	}

	result.Latency = latency
	return result
}
