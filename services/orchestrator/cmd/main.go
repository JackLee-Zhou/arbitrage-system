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
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if trading == nil {
			http.Error(w, `{"error":"trading_agent not connected"}`, http.StatusServiceUnavailable)
			return
		}

		resp, err := trading.EmergencyClose(r.Context(), req.Reason, req.Symbol)

		// 审计日志
		_ = audit.Write(r.Context(), domain.AuditEntry{
			Timestamp: time.Now(),
			Operator:  r.RemoteAddr,
			Action:    "emergency_close",
			Target:    req.Symbol,
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
