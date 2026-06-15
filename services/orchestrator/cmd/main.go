// orchestrator — 编排服务入口（API 网关 + WebSocket）
// 职责：REST API 网关 → gRPC 调用 trading_agent，聚合 Redis 数据，操作审计
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
	pkgregistry "github.com/arbitrage-system/pkg/registry"

	"github.com/arbitrage-system/orchestrator/internal/adapters"
	"github.com/arbitrage-system/orchestrator/internal/adapters/audit_writer"
	"github.com/arbitrage-system/orchestrator/internal/adapters/config"
	redisreader "github.com/arbitrage-system/orchestrator/internal/adapters/redis"
	"github.com/arbitrage-system/orchestrator/internal/adapters/trading_client"
	"github.com/arbitrage-system/orchestrator/internal/api"
	"github.com/arbitrage-system/orchestrator/internal/api/rest"
	"github.com/arbitrage-system/orchestrator/internal/api/websocket"
	"github.com/arbitrage-system/orchestrator/internal/domain"
)

const serviceVersion = "0.3.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[orchestrator] starting...")

	cfg := config.Load()
	log.Printf("[orchestrator] config: redis=%s trading_agent=%s port=%s",
		cfg.RedisAddr, cfg.TradingAgentGRPC, cfg.HTTPPort)

	// Redis（只读）
	redisClient := pkgredis.NewAdapter(cfg.RedisAddr)
	redisReader := redisreader.NewReader(redisClient)

	// trading_agent gRPC
	tradingClient, err := trading_client.NewClient(cfg.TradingAgentGRPC)
	if err != nil {
		log.Printf("[orchestrator] WARNING: trading_agent gRPC connection failed: %v (continuing without it)", err)
		tradingClient = nil
	}

	// 审计日志
	auditWriter := audit_writer.NewWriter(redisClient)

	// InfluxDB 只读（dashboard 历史数据查询）
	var influxReader *adapters.InfluxReader
	if cfg.InfluxToken != "" {
		influxReader = adapters.NewInfluxReader(cfg.InfluxURL, cfg.InfluxToken, cfg.InfluxOrg)
	} else {
		log.Println("[orchestrator] WARNING: INFLUXDB_TOKEN not set, /orders /trades /audit /history will be empty")
	}

	// 服务健康巡检（从 Redis 发现 + 探 /healthz → 缓存结果 → overview 暴露）
	healthPoller := adapters.NewHealthPoller(redisClient, 15*time.Second)
	go healthPoller.Start(context.Background())

	// 服务注册
	startTime := time.Now()
	host := cfg.ServiceHost
	if host == "" {
		host = cfg.ServiceName
	}
	reg, err := pkgregistry.Register(context.Background(), redisClient, pkgregistry.ServiceInfo{
		Name:         cfg.ServiceName,
		Version:      serviceVersion,
		StartedAt:    startTime,
		HealthURL:    fmt.Sprintf("%s:%s", host, cfg.HTTPPort),
		MetricsURL:   fmt.Sprintf("%s:%s", host, cfg.HTTPPort),
		Capabilities: []string{"rest_api", "websocket", "aggregation"},
	})
	if err != nil {
		log.Fatalf("[orchestrator] service registration failed: %v", err)
	}

	// WebSocket Hub
	wsHub := websocket.NewHub()

	// Snapshot provider: aggregates current state for new WS clients
	wsHub.SnapshotProvider = func() map[string]interface{} {
		ctx := context.Background()
		snap := map[string]interface{}{
			"system_status": map[string]interface{}{
				"mode":     "normal",
				"services": healthPoller.Snapshot(),
			},
		}
		if tradingClient != nil {
			if pos, err := tradingClient.GetPositions(ctx); err == nil {
				snap["positions"] = pos
			}
		} else if pos, err := redisReader.Position(ctx); err == nil {
			snap["positions"] = pos
		}
		if score, err := redisReader.AIScore(ctx, "MA2509"); err == nil {
			snap["scores"] = score
		}
		return snap
	}
	go wsHub.Run()

	// Price relay: poll Redis last_price:* → push WS deltas to contract:<symbol>
	priceRelay := adapters.NewPriceRelay(redisClient, wsHub)
	go priceRelay.Start(context.Background())

	// HTTP Handler
	mux := http.NewServeMux()

	healthChecker := &api.HealthChecker{
		ServiceName: cfg.ServiceName,
		Version:     serviceVersion,
		StartTime:   startTime,
		RedisPing:   redisReader.Ping,
		TradingPing: func(ctx context.Context) error {
			if tradingClient == nil {
				return fmt.Errorf("trading_agent not connected")
			}
			return tradingClient.Ping(ctx)
		},
	}
	mux.Handle("/healthz", healthChecker)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		uptime := int64(time.Since(startTime).Seconds())
		_, _ = fmt.Fprintf(w, "# orchestrator metrics\norchestrator_uptime_seconds %d\n", uptime)
	})

	mux.HandleFunc("/api/v1/overview", makeOverviewHandler(redisReader, tradingClient, healthPoller))
	mux.HandleFunc("/api/v1/positions", makePositionsHandler(redisReader, tradingClient))
	mux.HandleFunc("/api/v1/emergency_close", makeEmergencyCloseHandler(tradingClient, auditWriter))
	mux.HandleFunc("/api/v1/strategies", makeStrategiesHandler(tradingClient, auditWriter))
	mux.HandleFunc("/api/v1/risk", makeRiskHandler(tradingClient, auditWriter))
	mux.HandleFunc("/api/v1/contracts", makeContractsHandler(redisReader))
	mux.HandleFunc("/api/v1/contracts/", makeContractDetailHandler(redisReader, influxReader))
	mux.HandleFunc("/api/v1/ai/scores/current", makeAIScoresHandler(redisReader))
	mux.HandleFunc("/api/v1/services", makeServicesHandler(healthPoller))
	mux.HandleFunc("/api/v1/orders", makeOrdersHandler(influxReader))
	mux.HandleFunc("/api/v1/trades", makeTradesHandler(influxReader))
	mux.HandleFunc("/api/v1/ai/scores/history", makeAIScoresHistoryHandler(influxReader))
	mux.HandleFunc("/api/v1/audit", makeAuditHandler(influxReader))
	mux.HandleFunc("/ws", wsHub.HandleUpgrade)

	// 中间件链：日志 → 交易时段守卫
	handler := withMiddleware(mux)
	handler = rest.TradingSessionGuard(handler)

	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		log.Printf("[orchestrator] listening on :%s", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[orchestrator] HTTP server error: %v", err)
		}
	}()

	// 等待退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[orchestrator] received signal %v, shutting down...", sig)

	// 优雅关闭（各步骤独立超时）
	httpCtx, httpCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout/3)
	defer httpCancel()
	_ = httpServer.Shutdown(httpCtx)

	if tradingClient != nil {
		tradingClient.Close()
	}

	regCtx, regCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer regCancel()
	if err := reg.Shutdown(regCtx); err != nil {
		log.Printf("[orchestrator] registry shutdown error: %v", err)
	}
	_ = redisClient.Close()

	log.Println("[orchestrator] shutdown complete")
}

// ============================================================
// REST Handler
// ============================================================

func makeOverviewHandler(redis *redisreader.Reader, trading *trading_client.Client, health *adapters.HealthPoller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx := r.Context()

		result := map[string]interface{}{
			"status":      "ok",
			"system_mode": "normal",
			"services":    health.Snapshot(),
			"today_stats": map[string]interface{}{
				"orders": 0,
				"trades": 0,
				"pnl":    0.0,
			},
		}

		// AI Score
		if score, err := redis.AIScore(ctx, "MA2509"); err == nil {
			result["ai_score"] = score
		}

		// Engine status + positions from trading_agent
		if trading != nil {
			if status, err := trading.GetStatus(ctx); err == nil {
				result["engine"] = status
			}
			if positions, err := trading.GetPositions(ctx); err == nil {
				result["positions"] = positions
			}
		} else {
			// Redis fallback for positions
			if pos, err := redis.Position(ctx); err == nil {
				result["positions"] = pos
			}
		}

		_ = json.NewEncoder(w).Encode(result)
	}
}

func makePositionsHandler(redis *redisreader.Reader, trading *trading_client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx := r.Context()
		symbol := r.URL.Query().Get("symbol")

		var raw interface{}
		if trading != nil {
			if positions, err := trading.GetPositions(ctx); err == nil {
				raw = positions
			}
		}
		if raw == nil {
			if pos, err := redis.Position(ctx); err == nil {
				raw = pos
			}
		}
		if raw == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"positions": []interface{}{}})
			return
		}

		// If symbol filter requested, extract single position
		if symbol != "" {
			_ = json.NewEncoder(w).Encode(filterPosition(raw, symbol))
			return
		}

		_ = json.NewEncoder(w).Encode(raw)
	}
}

// filterPosition extracts a single position by symbol from gRPC/Redis response
func filterPosition(raw interface{}, symbol string) interface{} {
	// Handle map[string]interface{} with "positions" key (gRPC format)
	if m, ok := raw.(map[string]interface{}); ok {
		if positions, ok := m["positions"]; ok {
			if arr, ok := positions.([]interface{}); ok {
				for _, p := range arr {
					if pm, ok := p.(map[string]interface{}); ok {
						if pm["symbol"] == symbol {
							return pm
						}
					}
				}
			}
		}
	}
	return raw
}

func makeEmergencyCloseHandler(trading *trading_client.Client, audit *audit_writer.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Symbol string `json:"symbol"`
			Reason string `json:"reason"`
			Scope  string `json:"scope"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if trading == nil {
			http.Error(w, `{"error":"trading_agent not connected"}`, http.StatusServiceUnavailable)
			return
		}

		// scope 解析："single" → 传入 symbol；"all" → 传入空 symbol
		targetSymbol := req.Symbol
		switch req.Scope {
		case "all":
			targetSymbol = ""
		default:
			// "single" 或空值：使用 req.Symbol 原值
		}

		resp, err := trading.EmergencyClose(r.Context(), req.Reason, targetSymbol)

		// 审计日志
		target := targetSymbol
		if target == "" {
			target = "all"
		}
		_ = audit.Write(r.Context(), domain.AuditEntry{
			Timestamp: time.Now(),
			Operator:  r.RemoteAddr,
			Action:    "emergency_close",
			Target:    target,
			NewValue:  req.Reason,
			Result:    resultFrom(err),
			Error:     errStr(err),
			SourceIP:  r.RemoteAddr,
		})

		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func makeServicesHandler(health *adapters.HealthPoller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(health.Snapshot())
	}
}

func makeContractsHandler(redis *redisreader.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		var contracts []map[string]interface{}

		raw, err := redis.GetString(ctx, "instruments:all")
		if err == nil {
			var arr []map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				contracts = arr
			}
		}

		// Redis miss → 返回 mock 列表
		if contracts == nil {
			contracts = []map[string]interface{}{
				{"symbol": "MA2509", "name": "甲醇2509", "exchange": "DCE", "price": 2456.0, "change_pct": 0.35, "volume": 125630, "is_main": true},
				{"symbol": "MA2601", "name": "甲醇2601", "exchange": "DCE", "price": 2470.0, "change_pct": -0.12, "volume": 28640, "is_main": false},
				{"symbol": "MA2505", "name": "甲醇2505", "exchange": "DCE", "price": 2440.0, "change_pct": 0.08, "volume": 15200, "is_main": false},
				{"symbol": "RB2510", "name": "螺纹钢2510", "exchange": "SHFE", "price": 3780.0, "change_pct": 0.52, "volume": 342100, "is_main": true},
				{"symbol": "HC2510", "name": "热卷2510", "exchange": "SHFE", "price": 3910.0, "change_pct": 0.41, "volume": 89400, "is_main": true},
				{"symbol": "TA2509", "name": "PTA2509", "exchange": "CZCE", "price": 5880.0, "change_pct": -0.23, "volume": 234500, "is_main": true},
				{"symbol": "EG2509", "name": "乙二醇2509", "exchange": "DCE", "price": 4620.0, "change_pct": 0.15, "volume": 67200, "is_main": true},
				{"symbol": "PP2509", "name": "聚丙烯2509", "exchange": "DCE", "price": 7520.0, "change_pct": -0.08, "volume": 128900, "is_main": true},
				{"symbol": "L2509", "name": "聚乙烯2509", "exchange": "DCE", "price": 8250.0, "change_pct": 0.31, "volume": 94500, "is_main": true},
				{"symbol": "V2509", "name": "PVC2509", "exchange": "DCE", "price": 6020.0, "change_pct": -0.45, "volume": 78300, "is_main": true},
			}
		}

		// ?type=main 过滤
		if r.URL.Query().Get("type") == "main" {
			var filtered []map[string]interface{}
			for _, c := range contracts {
				if main, ok := c["is_main"].(bool); ok && main {
					filtered = append(filtered, c)
				}
			}
			contracts = filtered
		}

		_ = json.NewEncoder(w).Encode(contracts)
	}
}

func makeContractDetailHandler(redis *redisreader.Reader, influx *adapters.InfluxReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/contracts/")
		parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, `{"error":"symbol required"}`, http.StatusBadRequest)
			return
		}
		symbol := parts[0]
		subPath := ""
		if len(parts) > 1 {
			subPath = parts[1]
		}

		ctx := r.Context()

		switch subPath {
		case "intraday":
			if influx == nil {
				http.Error(w, `{"error":"influxdb not configured"}`, http.StatusServiceUnavailable)
				return
			}
			data, err := influx.QueryIntraday(ctx, symbol, 240)
			if err != nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}, "symbol": symbol})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": data, "symbol": symbol})

		case "kline":
			if influx == nil {
				http.Error(w, `{"error":"influxdb not configured"}`, http.StatusServiceUnavailable)
				return
			}
			period := r.URL.Query().Get("period")
			if period == "" {
				period = "1d"
			}
			data, err := influx.QueryKLine(ctx, symbol, period, time.Time{}, time.Time{}, 60)
			if err != nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}, "symbol": symbol})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": data, "symbol": symbol, "period": period})

		default: // snapshot
			fields, err := redis.HGetAll(ctx, "last_price:"+symbol)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"contract %s not found"}`, symbol), http.StatusNotFound)
				return
			}
			result := map[string]interface{}{
				"symbol":        symbol,
				"last_price":    fields["last_price"],
				"open":          fields["open"],
				"high":          fields["high"],
				"low":           fields["low"],
				"change_pct":    fields["change_pct"],
				"volume":        fields["volume"],
				"open_interest": fields["open_interest"],
				"updated_at":    fields["updated_at"],
			}
			if name, ok := fields["name"]; ok {
				result["name"] = name
			}
			_ = json.NewEncoder(w).Encode(result)
		}
	}
}

func makeAIScoresHandler(redis *redisreader.Reader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			symbol = "MA2509"
		}

		result := map[string]interface{}{
			"live":   nil,
			"shadow": nil,
		}

		// 读取 live 评分
		live, err := redis.AIScore(ctx, symbol)
		if err != nil {
			// 回退到默认
			if symbol != "MA2509" {
				live, err = redis.AIScore(ctx, "MA2509")
			}
		}
		if err == nil {
			result["live"] = live
		}

		// 读取 shadow 评分
		shadowRaw, err := redis.GetString(ctx, "ai_score:"+symbol+":shadow")
		if err == nil {
			var shadow map[string]interface{}
			if err := json.Unmarshal([]byte(shadowRaw), &shadow); err == nil {
				result["shadow"] = shadow
			}
		}

		_ = json.NewEncoder(w).Encode(result)
	}
}

func makeOrdersHandler(influx *adapters.InfluxReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if influx == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"orders": []interface{}{}})
			return
		}
		symbol := r.URL.Query().Get("symbol")
		data, err := influx.QueryOrders(r.Context(), symbol, 50)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"orders": []interface{}{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"orders": data})
	}
}

func makeTradesHandler(influx *adapters.InfluxReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if influx == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"trades": []interface{}{}})
			return
		}
		symbol := r.URL.Query().Get("symbol")
		data, err := influx.QueryTrades(r.Context(), symbol, 50)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"trades": []interface{}{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"trades": data})
	}
}

func makeAIScoresHistoryHandler(influx *adapters.InfluxReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if influx == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"scores": []interface{}{}})
			return
		}
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			symbol = "MA2509"
		}
		data, err := influx.QueryAIScoreHistory(r.Context(), symbol, time.Time{}, time.Time{}, 100)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"scores": []interface{}{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"scores": data, "symbol": symbol})
	}
}

func makeAuditHandler(influx *adapters.InfluxReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if influx == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"entries": []interface{}{}})
			return
		}
		data, err := influx.QueryAudit(r.Context(), time.Time{}, 100)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"entries": []interface{}{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"entries": data})
	}
}

func makeStrategiesHandler(trading *trading_client.Client, audit *audit_writer.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			if trading != nil {
				if status, err := trading.GetStatus(r.Context()); err == nil {
					_ = json.NewEncoder(w).Encode(status)
					return
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"strategies": []interface{}{}})

		case http.MethodPost:
			var req struct {
				Action string `json:"action"`
				Reason string `json:"reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
				return
			}
			if trading == nil {
				http.Error(w, `{"error":"trading_agent not connected"}`, http.StatusServiceUnavailable)
				return
			}

			var resp map[string]interface{}
			var err error
			switch req.Action {
			case "pause":
				resp, err = trading.PauseStrategy(r.Context(), req.Reason)
			case "resume":
				resp, err = trading.ResumeStrategy(r.Context())
			default:
				http.Error(w, `{"error":"unknown action"}`, http.StatusBadRequest)
				return
			}

			_ = audit.Write(r.Context(), domain.AuditEntry{
				Timestamp: time.Now(),
				Operator:  r.RemoteAddr,
				Action:    req.Action + "_strategy",
				NewValue:  req.Reason,
				Result:    resultFrom(err),
				Error:     errStr(err),
				SourceIP:  r.RemoteAddr,
			})

			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

func makeRiskHandler(trading *trading_client.Client, audit *audit_writer.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			if trading != nil {
				if status, err := trading.GetStatus(r.Context()); err == nil {
					_ = json.NewEncoder(w).Encode(status)
					return
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"risk_params": map[string]interface{}{}})

		case http.MethodPost:
			if trading == nil {
				http.Error(w, `{"error":"trading_agent not connected"}`, http.StatusServiceUnavailable)
				return
			}
			var params map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
				http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
				return
			}
			resp, err := trading.UpdateRiskParams(r.Context(), params)

			_ = audit.Write(r.Context(), domain.AuditEntry{
				Timestamp: time.Now(),
				Operator:  r.RemoteAddr,
				Action:    "update_risk",
				OldValue:  fmt.Sprintf("%v", params),
				Result:    resultFrom(err),
				Error:     errStr(err),
				SourceIP:  r.RemoteAddr,
			})

			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[orchestrator] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func resultFrom(err error) string {
	if err == nil {
		return "success"
	}
	return "failed"
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
