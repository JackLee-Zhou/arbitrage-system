// Package adapters — InfluxDB read-only queries for dashboard
package adapters

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

// InfluxReader provides read-only InfluxDB queries for dashboard endpoints
type InfluxReader struct {
	client influxdb2.Client
	org    string
}

// NewInfluxReader creates an InfluxReader
func NewInfluxReader(url, token, org string) *InfluxReader {
	client := influxdb2.NewClient(url, token)
	log.Printf("[influx-reader] connected to %s org=%s", url, org)
	return &InfluxReader{client: client, org: org}
}

// Close releases the InfluxDB client
func (r *InfluxReader) Close() {
	r.client.Close()
}

// validateSymbol ensures symbol matches expected pattern to prevent Flux injection
func validateSymbol(s string) error {
	matched, _ := regexp.MatchString(`^[A-Za-z0-9]{1,10}$`, s)
	if !matched {
		return fmt.Errorf("invalid symbol: %q", s)
	}
	return nil
}

// ---- Query helpers --------------------------------------------------------

// QueryOrders returns today's orders from order_history bucket
func (r *InfluxReader) QueryOrders(ctx context.Context, symbol string, limit int) ([]map[string]interface{}, error) {
	if symbol != "" {
		if err := validateSymbol(symbol); err != nil {
			return nil, err
		}
	}
	filter := ""
	if symbol != "" {
		filter = fmt.Sprintf(`|> filter(fn: (r) => r["symbol"] == "%s")`, symbol)
	}
	if limit <= 0 {
		limit = 50
	}
	flux := fmt.Sprintf(`
		from(bucket: "order_history")
		|> range(start: -24h)
		%s
		|> sort(columns: ["_time"], desc: true)
		|> limit(n: %d)
	`, filter, limit)
	return r.executeQuery(ctx, flux)
}

// QueryTrades returns today's filled trades
func (r *InfluxReader) QueryTrades(ctx context.Context, symbol string, limit int) ([]map[string]interface{}, error) {
	if symbol != "" {
		if err := validateSymbol(symbol); err != nil {
			return nil, err
		}
	}
	filter := `|> filter(fn: (r) => r["_field"] == "status" or r["_field"] == "filled_volume")`
	if symbol != "" {
		filter += fmt.Sprintf(`\n|> filter(fn: (r) => r["symbol"] == "%s")`, symbol)
	}
	if limit <= 0 {
		limit = 50
	}
	flux := fmt.Sprintf(`
		from(bucket: "order_history")
		|> range(start: -24h)
		%s
		|> filter(fn: (r) => r["status"] == "filled" or r["status"] == "partial")
		|> sort(columns: ["_time"], desc: true)
		|> limit(n: %d)
	`, filter, limit)
	return r.executeQuery(ctx, flux)
}

// QueryAIScoreHistory returns AI score history for a symbol
func (r *InfluxReader) QueryAIScoreHistory(ctx context.Context, symbol string, from, to time.Time, limit int) ([]map[string]interface{}, error) {
	if symbol != "" {
		if err := validateSymbol(symbol); err != nil {
			return nil, err
		}
	}
	if symbol == "" {
		symbol = "MA2509"
	}
	if from.IsZero() {
		from = time.Now().Add(-24 * time.Hour)
	}
	if to.IsZero() {
		to = time.Now()
	}
	if limit <= 0 {
		limit = 100
	}
	flux := fmt.Sprintf(`
		from(bucket: "ai_score_history")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r["symbol"] == "%s")
		|> sort(columns: ["_time"], desc: false)
		|> limit(n: %d)
	`, from.Format(time.RFC3339), to.Format(time.RFC3339), symbol, limit)
	return r.executeQuery(ctx, flux)
}

// QueryIntraday returns minute-level OHLC for a symbol (today)
func (r *InfluxReader) QueryIntraday(ctx context.Context, symbol string, limit int) ([]map[string]interface{}, error) {
	if symbol != "" {
		if err := validateSymbol(symbol); err != nil {
			return nil, err
		}
	}
	if symbol == "" {
		symbol = "MA2509"
	}
	if limit <= 0 {
		limit = 240 // 4 hours of 1-min data
	}
	flux := fmt.Sprintf(`
		from(bucket: "ma_1min")
		|> range(start: -8h)
		|> filter(fn: (r) => r["symbol"] == "%s")
		|> sort(columns: ["_time"], desc: false)
		|> limit(n: %d)
	`, symbol, limit)
	return r.executeQuery(ctx, flux)
}

// QueryKLine returns daily/weekly/monthly OHLC bars
func (r *InfluxReader) QueryKLine(ctx context.Context, symbol string, period string, from, to time.Time, limit int) ([]map[string]interface{}, error) {
	if symbol != "" {
		if err := validateSymbol(symbol); err != nil {
			return nil, err
		}
	}
	if symbol == "" {
		symbol = "MA2509"
	}
	if from.IsZero() {
		from = time.Now().Add(-30 * 24 * time.Hour)
	}
	if to.IsZero() {
		to = time.Now()
	}
	if limit <= 0 {
		limit = 60
	}

	window := "1d"
	switch period {
	case "1w", "week", "weekly":
		window = "1w"
	case "1mo", "month", "monthly":
		window = "1mo"
	}

	flux := fmt.Sprintf(`
		from(bucket: "ma_1min")
		|> range(start: %s, stop: %s)
		|> filter(fn: (r) => r["symbol"] == "%s")
		|> aggregateWindow(every: %s, fn: first, createEmpty: false)
		|> sort(columns: ["_time"], desc: false)
		|> limit(n: %d)
	`, from.Format(time.RFC3339), to.Format(time.RFC3339), symbol, window, limit)
	return r.executeQuery(ctx, flux)
}

// QueryAudit returns recent audit log entries
func (r *InfluxReader) QueryAudit(ctx context.Context, from time.Time, limit int) ([]map[string]interface{}, error) {
	if from.IsZero() {
		from = time.Now().Add(-7 * 24 * time.Hour)
	}
	if limit <= 0 {
		limit = 100
	}
	flux := fmt.Sprintf(`
		from(bucket: "audit_log")
		|> range(start: %s)
		|> sort(columns: ["_time"], desc: true)
		|> limit(n: %d)
	`, from.Format(time.RFC3339), limit)
	return r.executeQuery(ctx, flux)
}

const (
	defaultQueryTimeout = 5 * time.Second
	maxRowsPerQuery     = 2000
)

func (r *InfluxReader) executeQuery(ctx context.Context, flux string) ([]map[string]interface{}, error) {
	// Per-query timeout: prevents a slow Flux query from hogging the goroutine forever.
	// Heavy queries (120d K-line ~3-8s) get the full budget; light queries finish early.
	queryCtx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
	defer cancel()

	queryAPI := r.client.QueryAPI(r.org)
	result, err := queryAPI.Query(queryCtx, flux)
	if err != nil {
		return nil, fmt.Errorf("influx query failed: %w", err)
	}
	defer func() {
		// Close drains the remaining result stream and releases the HTTP connection.
		// Must always be called, even on error paths, to avoid goroutine/connection leaks.
		_ = result.Close()
	}()

	var rows []map[string]interface{}
	for result.Next() {
		// Respect context cancellation (client disconnected / timeout)
		select {
		case <-queryCtx.Done():
			return rows, fmt.Errorf("query cancelled: %w", queryCtx.Err())
		default:
		}

		// Hard cap to prevent OOM from unexpectedly large result sets
		if len(rows) >= maxRowsPerQuery {
			break
		}

		record := result.Record()
		row := map[string]interface{}{
			"time":  record.Time().UnixMilli(),
			"field": record.Field(),
			"value": record.Value(),
		}
		for k, v := range record.Values() {
			if k != "_time" && k != "_field" && k != "_value" && k != "_measurement" && k != "_start" && k != "_stop" && k != "result" && k != "table" {
				row[k] = v
			}
		}
		rows = append(rows, row)
	}

	if err := result.Err(); err != nil {
		return rows, fmt.Errorf("influx read error: %w", err)
	}
	return rows, nil
}
