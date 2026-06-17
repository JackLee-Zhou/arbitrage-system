// trading_agent/adapters/llm_client — HTTP 客户端 -> llm_scorer /emergency_analysis
//
// 检测到剧烈波动（价格波动 > 1% / 量比 > 3 / 单边比 > 0.85）时，
// 调用 llm_scorer 的 /emergency_analysis 端点进行紧急分析。
// 防抖逻辑由上层调用者（domain engine）控制，与本层无关。
package llm_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// ============================================================
// 常量
// ============================================================
const (
	defaultTimeout = 30 * time.Second
	defaultBaseURL = "http://llm_scorer:5000"
	emergencyPath  = "/emergency_analysis"
)

// ============================================================
// 数据模型
// ============================================================

// EmergencyRequest 紧急分析请求参数。
// POST /emergency_analysis 的请求体。
type EmergencyRequest struct {
	Symbol        string              `json:"symbol"`
	TriggerReason string              `json:"trigger_reason"` // "price_spike" / "volume_surge" / "boll_breakout"
	Volatility    EmergencyVolatility `json:"volatility"`
}

// EmergencyVolatility 波动率指标
type EmergencyVolatility struct {
	PriceChangePct   float64 `json:"price_change_pct"`
	VolumeMultiplier float64 `json:"volume_multiplier"`
	OneSidedRatio    float64 `json:"one_sided_ratio"`
}

// EmergencyResponse 紧急分析响应（对应 contracts/ai_score.proto 的 EmergencyAIScore）
type EmergencyResponse struct {
	Base AIScore `json:"base"`

	TriggerReason string `json:"trigger_reason"`

	Volatility struct {
		PriceChangePct1m float64 `json:"price_change_pct_1m"`
		VolumeMultiplier float64 `json:"volume_multiplier"`
		OneSidedRatio    float64 `json:"one_sided_ratio"`
	} `json:"volatility"`

	BreakingNews     []string `json:"breaking_news,omitempty"`
	SuggestedAction  string   `json:"suggested_action"`
	ActionConfidence float64  `json:"action_confidence"`
}

// AIScore 基础评分结构（对应 contracts/ai_score.proto 的 AIScore）
type AIScore struct {
	Symbol     string   `json:"symbol"`
	Timestamp  int64    `json:"timestamp"`
	ExpiresAt  int64    `json:"expires_at"`
	Score      float64  `json:"score"`
	Confidence float64  `json:"confidence"`
	Horizon    string   `json:"horizon"`
	Regime     string   `json:"regime"`
	KeyDrivers []string `json:"key_drivers,omitempty"`
	RawText    string   `json:"raw_text,omitempty"`
}

// ============================================================
// LLMClient — llm_scorer HTTP 客户端
// ============================================================

// LLMClient 封装对 llm_scorer 服务的 HTTP 调用。
// 紧急分析 timeout 30s（比常规 60s 短，快速响应剧烈波动）。
type LLMClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewLLMClient 创建 LLM 客户端。
// baseURL: llm_scorer 地址，默认 http://llm_scorer:5000
func NewLLMClient(baseURL string) *LLMClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &LLMClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// RequestEmergency 触发紧急分析。
// 5 分钟防抖由上层 domain engine 控制，此方法不做防抖。
//
// POST /emergency_analysis
// Body: EmergencyRequest 的 JSON 序列化
// Response: EmergencyResponse（对应 contracts/ai_score.proto 的 EmergencyAIScore）
func (c *LLMClient) RequestEmergency(ctx context.Context, req EmergencyRequest) (*EmergencyResponse, error) {
	endpoint := c.baseURL + emergencyPath

	log.Printf("[llm_client] RequestEmergency: symbol=%s trigger=%s price_change=%.2f%% volume_mult=%.1f",
		req.Symbol, req.TriggerReason,
		req.Volatility.PriceChangePct*100, req.Volatility.VolumeMultiplier)

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("json marshal emergency request failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("emergency analysis http call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("emergency analysis returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result EmergencyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("json decode emergency response failed: %w", err)
	}

	log.Printf("[llm_client] Emergency result: score=%.3f confidence=%.2f action=%s",
		result.Base.Score, result.Base.Confidence, result.SuggestedAction)

	return &result, nil
}

// Ping 检查 llm_scorer 连通性（供 healthz 使用）
func (c *LLMClient) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(pingCtx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("create ping request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("llm_scorer ping failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llm_scorer ping returned status %d", resp.StatusCode)
	}

	return nil
}
