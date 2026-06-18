// trading_agent/adapters/influxdb — InfluxDB 只读查询器
//
// ⚠️ 重要约束：trading_agent 正常运行时**不直接写** InfluxDB，
// 所有持久化通过调用 StateStore.PublishXxx() 写 Redis Stream，由 db_writer 消费后写入。
// 唯一例外：启动三步对账时从 InfluxDB 直接查询最近数据（兜底恢复用）。
//
// 查询场景：
//   - QueryLatestPosition — 启动对账 Step 2，从 order_history bucket 查询最近持仓快照
//   - QueryRecentOrders   — 启动时补齐 Redis 缺失的报单状态
//   - Ping                 — 供 healthz 端点使用
package influxdb

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

// ============================================================
// 常量
// ============================================================
const (
	defaultOrg    = "arbitrage"
	defaultBucket = "order_history"
)

// ============================================================
// 数据模型
// ============================================================

// Position 持仓快照（独立定义，避免包间依赖）
type Position struct {
	Symbol    string  `json:"symbol"`
	Direction string  `json:"direction"`
	Volume    int     `json:"volume"`
	AvgPrice  float64 `json:"avg_price"`
	LastPrice float64 `json:"last_price"`
	FloatPnL  float64 `json:"float_pnl"`
}

// OrderRecord 报单记录
type OrderRecord struct {
	OrderID     string  `json:"order_id"`
	Symbol      string  `json:"symbol"`
	Direction   string  `json:"direction"`
	Offset      string  `json:"offset"`
	Volume      int32   `json:"volume"`
	Price       float64 `json:"price"`
	OrderType   string  `json:"order_type"`
	Status      string  `json:"status"`
	FilledPrice float64 `json:"filled_price"`
	FilledVol   int32   `json:"filled_volume"`
	Timestamp   int64   `json:"timestamp"`
}

// ============================================================
// InfluxReader — InfluxDB 只读查询器
// ============================================================

// InfluxReader 通过 InfluxDB 客户端查询历史数据。
// 仅用于启动三步对账兜底恢复，不用于正常运行时。
type InfluxReader struct {
	client influxdb2.Client
	org    string
	bucket string
}

// NewInfluxReader 创建 InfluxDB 只读查询器。
// url: http://influxdb:8086
// token: InfluxDB API token（具有读权限即可）
// org: 组织名，默认 "arbitrage"
func NewInfluxReader(url, token, org string) *InfluxReader {
	if org == "" {
		org = defaultOrg
	}
	client := influxdb2.NewClient(url, token)
	return &InfluxReader{
		client: client,
		org:    org,
		bucket: defaultBucket,
	}
}

// Close 释放 InfluxDB 连接
func (r *InfluxReader) Close() {
	r.client.Close()
}

// QueryLatestPosition 查询最近的持仓快照。
// 用于启动对账 Step 2：Redis 无持仓时从 InfluxDB 恢复。
//
// Bucket: order_history, measurement: position_snapshot
// 按时间倒序取最近 1 条。
func (r *InfluxReader) QueryLatestPosition(ctx context.Context, symbol string) (*Position, error) {
	fluxQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: -7d)
  |> filter(fn: (r) => r._measurement == "position_snapshot" and r.symbol == "%s")
  |> pivot(rowKey: ["_time"], columnKey: ["_field"], valueColumn: "_value")
  |> last()
`, r.bucket, symbol)

	return r.queryPosition(ctx, fluxQuery, symbol)
}

// QueryRecentOrders 查询最近 N 小时的报单记录。
// 用于启动时补齐 Redis 缺失的报单状态。
//
// Bucket: order_history, measurement: order
// 按时间倒序返回。
func (r *InfluxReader) QueryRecentOrders(ctx context.Context, symbol string, since time.Duration) ([]OrderRecord, error) {
	sinceHours := int(since.Hours())
	if sinceHours < 1 {
		sinceHours = 1
	}

	fluxQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: -%dh)
  |> filter(fn: (r) => r._measurement == "order" and r.symbol == "%s")
  |> pivot(rowKey: ["_time"], columnKey: ["_field"], valueColumn: "_value")
  |> sort(columns: ["_time"], desc: true)
`, r.bucket, sinceHours, symbol)

	return r.queryOrders(ctx, fluxQuery, symbol)
}

// Ping 检查 InfluxDB 连通性（供 healthz 使用）
func (r *InfluxReader) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := r.client.Health(pingCtx)
	if err != nil {
		return fmt.Errorf("influxdb health check failed: %w", err)
	}
	return nil
}

// ============================================================
// 内部查询方法
// ============================================================

// queryPosition 执行持仓快照查询，返回第一条结果
func (r *InfluxReader) queryPosition(ctx context.Context, fluxQuery string, symbol string) (*Position, error) {
	queryAPI := r.client.QueryAPI(r.org)

	result, err := queryAPI.Query(ctx, fluxQuery)
	if err != nil {
		return nil, fmt.Errorf("influxdb query position failed: %w", err)
	}
	defer func() { _ = result.Close() }()

	for result.Next() {
		if result.Record() == nil {
			continue
		}
		record := result.Record()

		pos := &Position{
			Symbol: symbol,
		}

		if d := record.ValueByKey("direction"); d != nil {
			pos.Direction = fmt.Sprintf("%v", d)
		}
		if v := record.ValueByKey("volume"); v != nil {
			pos.Volume = toInt(v)
		}
		if p := record.ValueByKey("avg_price"); p != nil {
			pos.AvgPrice = toFloat(p)
		}
		if p := record.ValueByKey("last_price"); p != nil {
			pos.LastPrice = toFloat(p)
		}
		if p := record.ValueByKey("float_pnl"); p != nil {
			pos.FloatPnL = toFloat(p)
		}

		return pos, nil
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("influxdb query position iteration failed: %w", err)
	}

	log.Printf("[influxdb] no position found for %s in InfluxDB", symbol)
	return nil, nil
}

// queryOrders 执行报单查询，返回所有结果
func (r *InfluxReader) queryOrders(ctx context.Context, fluxQuery string, symbol string) ([]OrderRecord, error) {
	queryAPI := r.client.QueryAPI(r.org)

	result, err := queryAPI.Query(ctx, fluxQuery)
	if err != nil {
		return nil, fmt.Errorf("influxdb query orders failed: %w", err)
	}
	defer func() { _ = result.Close() }()

	var orders []OrderRecord

	for result.Next() {
		if result.Record() == nil {
			continue
		}
		record := result.Record()

		order := OrderRecord{
			Symbol: symbol,
		}

		if id := record.ValueByKey("order_id"); id != nil {
			order.OrderID = fmt.Sprintf("%v", id)
		}
		if d := record.ValueByKey("direction"); d != nil {
			order.Direction = fmt.Sprintf("%v", d)
		}
		if o := record.ValueByKey("offset"); o != nil {
			order.Offset = fmt.Sprintf("%v", o)
		}
		if v := record.ValueByKey("volume"); v != nil {
			order.Volume = int32(toInt(v))
		}
		if p := record.ValueByKey("price"); p != nil {
			order.Price = toFloat(p)
		}
		if ot := record.ValueByKey("order_type"); ot != nil {
			order.OrderType = fmt.Sprintf("%v", ot)
		}
		if s := record.ValueByKey("status"); s != nil {
			order.Status = fmt.Sprintf("%v", s)
		}
		if fp := record.ValueByKey("filled_price"); fp != nil {
			order.FilledPrice = toFloat(fp)
		}
		if fv := record.ValueByKey("filled_volume"); fv != nil {
			order.FilledVol = int32(toInt(fv))
		}

		order.Timestamp = record.Time().UnixMilli()
		orders = append(orders, order)
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("influxdb query orders iteration failed: %w", err)
	}

	return orders, nil
}

// ============================================================
// 数值转换工具
// ============================================================

// toFloat 安全转换 interface{} 到 float64
func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err == nil {
			return f
		}
		return 0
	case int64:
		return float64(val)
	case int:
		return float64(val)
	default:
		return 0
	}
}

// toInt 安全转换 interface{} 到 int
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int64:
		return int(val)
	case int:
		return val
	case float64:
		return int(val)
	case string:
		i, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return int(i)
		}
		return 0
	default:
		return 0
	}
}
