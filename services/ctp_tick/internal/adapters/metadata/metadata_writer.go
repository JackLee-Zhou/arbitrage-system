// ctp_tick/adapters/metadata — 合约元数据持久化适配器
package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

// ============================================================
// 常量
// ============================================================

const (
	// InstrumentsAllKey 全市场合约列表（JSON List）
	InstrumentsAllKey = "instruments:all"
	// InstrumentPrefix 单个合约元数据前缀
	InstrumentPrefix = "instrument:"
	// InfluxDBWriteTimeout 写入超时
	InfluxDBWriteTimeout = 10 * time.Second
)

// ============================================================
// Writer — 合约元数据写入器
// ============================================================

// Writer 合约元数据写入器（Redis + InfluxDB）
type Writer struct {
	redisClient pkgredis.RedisClient
	influxURL   string // InfluxDB HTTP API 地址，如 "http://influxdb:8086"
	influxToken string // InfluxDB 认证 token
	influxOrg   string // InfluxDB 组织
}

// NewWriter 创建合约元数据写入器
func NewWriter(client pkgredis.RedisClient, influxURL, influxToken, influxOrg string) *Writer {
	return &Writer{
		redisClient: client,
		influxURL:   influxURL,
		influxToken: influxToken,
		influxOrg:   influxOrg,
	}
}

// WriteAll 将合约元数据写入 Redis + InfluxDB
// 可独立调用（例如每月主力切换时再写一次 InfluxDB）
func (w *Writer) WriteAll(ctx context.Context, instruments []domain.InstrumentMetadata) error {
	if len(instruments) == 0 {
		log.Println("[metadata] no instruments to write, skipping")
		return nil
	}

	// 写 Redis
	if err := w.writeRedis(ctx, instruments); err != nil {
		return fmt.Errorf("write instruments to redis: %w", err)
	}

	// 写 InfluxDB
	if err := w.writeInfluxDB(ctx, instruments); err != nil {
		return fmt.Errorf("write instruments to influxdb: %w", err)
	}

	log.Printf("[metadata] wrote %d instruments to redis + influxdb", len(instruments))
	return nil
}

// ============================================================
// Redis 写入
// ============================================================

func (w *Writer) writeRedis(ctx context.Context, instruments []domain.InstrumentMetadata) error {
	// 1. 写 instruments:all — 全量 JSON 列表
	listJSON, err := json.Marshal(instruments)
	if err != nil {
		return fmt.Errorf("marshal instruments:all: %w", err)
	}
	if err := w.redisClient.Set(ctx, InstrumentsAllKey, string(listJSON), 0); err != nil {
		return fmt.Errorf("set %s: %w", InstrumentsAllKey, err)
	}
	log.Printf("[metadata] wrote %d instruments to redis key %s", len(instruments), InstrumentsAllKey)

	// 2. 逐合约写 instrument:<symbol> — 单个 JSON
	for _, inst := range instruments {
		singleJSON, err := json.Marshal(inst)
		if err != nil {
			log.Printf("[metadata] marshal instrument %s failed: %v", inst.Symbol, err)
			continue
		}
		key := InstrumentPrefix + inst.Symbol
		if err := w.redisClient.Set(ctx, key, string(singleJSON), 0); err != nil {
			log.Printf("[metadata] set %s failed: %v", key, err)
			continue
		}
	}
	log.Printf("[metadata] wrote individual instrument keys to redis")

	return nil
}

// ============================================================
// InfluxDB 写入（使用 HTTP API 直写 Line Protocol）
// ============================================================

// writeInfluxDB 合约元数据写入 InfluxDB instruments bucket
// 使用 InfluxDB HTTP API v2 直接写 Line Protocol
// bucket: "instruments", retention 永久
// 格式: instruments,symbol=XXX,exchange=XXX product_id="XXX",... timestamp
func (w *Writer) writeInfluxDB(ctx context.Context, instruments []domain.InstrumentMetadata) error {
	if w.influxURL == "" {
		log.Println("[metadata] InfluxDB URL not configured, skipping influx write")
		return nil
	}

	var buf bytes.Buffer
	now := time.Now().UnixNano()

	for _, inst := range instruments {
		// 如果必要字段为空，跳过该合约
		if inst.Symbol == "" || inst.Exchange == "" {
			continue
		}

		// 构建 Line Protocol 记录
		tags := fmt.Sprintf("symbol=%s,exchange=%s,product_id=%s",
			escapeInfluxTag(inst.Symbol),
			escapeInfluxTag(inst.Exchange),
			escapeInfluxTag(inst.ProductID))

		fields := fmt.Sprintf("product_name_cn=%s,contract_year=%di,contract_month=%di,multiplier=%s,price_tick=%s,margin_ratio=%s",
			escapeInfluxField(inst.ProductNameCN),
			inst.ContractYear,
			inst.ContractMonth,
			formatInfluxFloat(inst.Multiplier),
			formatInfluxFloat(inst.PriceTick),
			formatInfluxFloat(inst.LongMarginRatio))

		line := fmt.Sprintf("instruments,%s %s %d\n", tags, fields, now)
		buf.WriteString(line)
	}

	if buf.Len() == 0 {
		return nil
	}

	// POST 到 InfluxDB /api/v2/write
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v2/write?org=%s&bucket=instruments&precision=ns",
			w.influxURL, w.influxOrg),
		&buf)
	if err != nil {
		return fmt.Errorf("create influxdb request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+w.influxToken)
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	client := &http.Client{Timeout: InfluxDBWriteTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("influxdb write request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("influxdb write HTTP %d", resp.StatusCode)
	}

	log.Printf("[metadata] wrote %d instruments to influxdb bucket 'instruments'", len(instruments))
	return nil
}

// ============================================================
// Line Protocol 辅助函数
// ============================================================

// escapeInfluxTag 转义 InfluxDB tag value 中的特殊字符
func escapeInfluxTag(s string) string {
	if s == "" {
		return "unknown"
	}
	// Tag value 需要转义的反斜杠、逗号、等号、空格
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == ',' || c == '=' || c == ' ' {
			result = append(result, '\\', c)
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

// escapeInfluxField 转义 InfluxDB 字符串 field value
// 字符串需双引号包裹，内部的反斜杠和双引号需转义
func escapeInfluxField(s string) string {
	if s == "" {
		return `""`
	}
	escaped := make([]byte, 0, len(s)+2)
	escaped = append(escaped, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == '"' {
			escaped = append(escaped, '\\', c)
		} else {
			escaped = append(escaped, c)
		}
	}
	escaped = append(escaped, '"')
	return string(escaped)
}

// formatInfluxFloat 将 float64 格式化为 InfluxDB 兼容浮点值
func formatInfluxFloat(v float64) string {
	return fmt.Sprintf("%g", v)
}
