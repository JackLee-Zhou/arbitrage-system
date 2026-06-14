// db_writer -- InfluxDB 串行写入服务入口
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
	pkgregistry "github.com/arbitrage-system/pkg/registry"

	"github.com/arbitrage-system/db_writer/internal/adapters/alert"
	"github.com/arbitrage-system/db_writer/internal/adapters/config"
	"github.com/arbitrage-system/db_writer/internal/adapters/influxdb"
	redisadapter "github.com/arbitrage-system/db_writer/internal/adapters/redis"
	"github.com/arbitrage-system/db_writer/internal/api"
	"github.com/arbitrage-system/db_writer/internal/domain"
)

// ============================================================
// 常量
// ============================================================

const serviceVersion = "1.0.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[db_writer] starting...")

	// ---- Step 1: 加载配置 ----
	cfg := config.Load()
	log.Printf("[db_writer] config: service=%s redis=%s influx=%s batch=%d/%v retries=%d port=%d",
		cfg.ServiceName, cfg.RedisAddr, cfg.InfluxDBURL,
		cfg.BatchSize, cfg.BatchInterval, cfg.MaxRetries, cfg.HealthPort)

	// ---- Step 2: 连接 Redis ----
	redisClient := pkgredis.NewAdapter(cfg.RedisAddr)
	if err := redisClient.Ping(context.Background()); err != nil {
		log.Printf("[db_writer] warning: initial Redis ping failed: %v", err)
	}

	// ---- Step 3: 连接 InfluxDB ----
	retryPolicy := domain.RetryPolicy{
		MaxRetries:        cfg.MaxRetries,
		InitialBackoff:    domain.InitialBackoff,
		BackoffMultiplier: domain.BackoffMultiplier,
	}
	influxWriter, err := influxdb.NewWriter(cfg.InfluxDBURL, cfg.InfluxDBToken, cfg.InfluxDBOrg, retryPolicy, cfg.InfluxWriteTimeout)
	if err != nil {
		log.Fatalf("[db_writer] failed to create InfluxDB writer: %v", err)
	}
	defer influxWriter.Close()

	// ---- Step 4: 创建指标收集器 ----
	metrics := api.NewMetrics(config.AllBuckets(), config.AllStreams())

	// ---- Step 5: 注册服务 ----
	startTime := time.Now()
	reg, err := pkgregistry.Register(context.Background(), redisClient, pkgregistry.ServiceInfo{
		Name:         cfg.ServiceName,
		Version:      serviceVersion,
		StartedAt:    startTime,
		HealthURL:    fmt.Sprintf(":%d", cfg.HealthPort),
		MetricsURL:   fmt.Sprintf(":%d", cfg.HealthPort),
		Capabilities: []string{"influxdb_writer"},
	})
	if err != nil {
		log.Printf("[db_writer] warning: service registration failed: %v (continuing)", err)
	}

	// ---- Step 6: 创建消费者组 ----
	batchRule := domain.BatchRule{
		MaxSize: cfg.BatchSize,
		MaxWait: cfg.BatchInterval,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建所有消费组（忽略已存在错误）
	for _, stream := range cfg.Streams {
		_ = redisClient.XGroupCreate(ctx, stream, redisadapter.ConsumerGroupName, "$")
	}

	// ---- Step 7: 启动消费者（每个 stream 一个 goroutine）----
	type consumerInfo struct {
		consumer *redisadapter.StreamConsumer
		stream   string
		bucket   string
	}

	consumers := make([]consumerInfo, 0, len(cfg.Streams))
	for _, stream := range cfg.Streams {
		bucket := config.StreamToBucket(stream)

		writeTotal := metrics.WritesTotal[bucket]
		if writeTotal == nil {
			writeTotal = &atomic.Int64{}
		}
		writeErrors := metrics.WriteErrorsTotal[bucket]
		if writeErrors == nil {
			writeErrors = &atomic.Int64{}
		}
		writeLatency := metrics.WriteLatencyP50[bucket]
		if writeLatency == nil {
			writeLatency = &atomic.Int64{}
		}

		consumer := redisadapter.NewStreamConsumer(
			redisClient, stream, batchRule, influxWriter,
			writeTotal, writeErrors, writeLatency,
			cfg.XReadBlockMs, cfg.XReadBatchSize, cfg.DLQWriteTimeout, cfg.DLQConcurrency,
		)
		consumers = append(consumers, consumerInfo{
			consumer: consumer,
			stream:   stream,
			bucket:   bucket,
		})

		go consumer.Consume(ctx)
		log.Printf("[db_writer] consumer started: stream=%s bucket=%s", stream, bucket)
	}

	// ---- Step 8: 启动死信队列监控 ----
	alerter := alert.NewDingtalkAlerter(cfg.DingtalkWebhook)
	go monitorDLQ(ctx, redisClient, alerter, metrics, config.AllBuckets(), cfg.DLQMonitorInterval)

	// ---- Step 9: 启动 HTTP 服务 ----
	healthHandler := &api.HealthHandler{
		ServiceName: cfg.ServiceName,
		Version:     serviceVersion,
		StartTime:   startTime,
		Funcs: api.HealthCheckFuncs{
			RedisPing: func(ctx context.Context) error {
				pingCtx, pingCancel := context.WithTimeout(ctx, cfg.RedisPingTimeout)
				defer pingCancel()
				return redisClient.Ping(pingCtx)
			},
			InfluxDBPing: func(ctx context.Context) error {
				pingCtx, pingCancel := context.WithTimeout(ctx, cfg.InfluxPingTimeout)
				defer pingCancel()
				return influxWriter.Ping(pingCtx)
			},
		},
	}

	mux := http.NewServeMux()
	mux.Handle("/healthz", healthHandler)
	mux.Handle("/metrics", metrics)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HealthPort),
		Handler:      mux,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}
	go func() {
		log.Printf("[db_writer] health server on :%d", cfg.HealthPort)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[db_writer] HTTP server error: %v", err)
		}
	}()

	// ---- Step 10: 等待退出信号 ----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("[db_writer] received signal %v, shutting down...", sig)
	case <-ctx.Done():
	}

	// ---- Step 11: 优雅关停 ----
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	// 先取消全局 context（通知所有后台 goroutine）
	cancel()

	// 停止所有消费者（并行，每个 consumer 有独立超时）
	log.Println("[db_writer] stopping consumers...")
	var wg sync.WaitGroup
	for _, ci := range consumers {
		wg.Add(1)
		go func(c *redisadapter.StreamConsumer) {
			defer wg.Done()
			stopCtx, stopCancel := context.WithTimeout(shutdownCtx, cfg.ConsumerStopTimeout)
			defer stopCancel()
			if err := c.Stop(stopCtx); err != nil {
				log.Printf("[db_writer] consumer stop error: %v", err)
			}
		}(ci.consumer)
	}
	wg.Wait()

	// 注销服务注册
	if reg != nil {
		if err := reg.Shutdown(shutdownCtx); err != nil {
			log.Printf("[db_writer] registry shutdown error: %v", err)
		}
	}

	// 停止 HTTP 服务
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[db_writer] HTTP shutdown timeout, force exit")
	}

	// 关闭 Redis
	_ = redisClient.Close()

	log.Println("[db_writer] shutdown complete")
}

// ============================================================
// DLQ 监控 goroutine
// ============================================================

// monitorDLQ 定期检查所有死信队列大小
// 超过阈值则推钉钉告警，同时更新指标
func monitorDLQ(ctx context.Context, redisClient pkgredis.RedisClient, alerter *alert.Alerter, metrics *api.Metrics, buckets []string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Println("[dlq_monitor] started")

	// 立即检查一次
	checkDLQ(ctx, redisClient, alerter, metrics, buckets)

	for {
		select {
		case <-ctx.Done():
			log.Println("[dlq_monitor] stopped")
			return
		case <-ticker.C:
			checkDLQ(ctx, redisClient, alerter, metrics, buckets)
		}
	}
}

// checkDLQ 检查并更新 DLQ 状态
func checkDLQ(ctx context.Context, redisClient pkgredis.RedisClient, alerter *alert.Alerter, metrics *api.Metrics, buckets []string) {
	dlq := redisadapter.NewDLQWriter(redisClient)

	for _, bucket := range buckets {
		size, err := dlq.Size(ctx, bucket)
		if err != nil {
			log.Printf("[dlq_monitor] failed to check dlq size for bucket %s: %v", bucket, err)
			continue
		}

		// 更新指标
		if counter, ok := metrics.DLQSize[bucket]; ok {
			counter.Store(size)
		}

		if size > 0 {
			log.Printf("[dlq_monitor] bucket %s has %d dead letters", bucket, size)

			// 超过阈值推钉钉
			if domain.ShouldAlert(int(size)) {
				title := fmt.Sprintf("db_writer 死信告警 - %s", bucket)
				text := fmt.Sprintf(
					"## db_writer 死信队列告警\n\n"+
						"- **Bucket**: %s\n"+
						"- **死信数量**: %d\n"+
						"- **阈值**: %d\n"+
						"- **时间**: %s\n\n"+
						"> 请及时处理死信数据，避免数据丢失。",
					bucket, size, domain.DLQAlertThreshold, time.Now().Format(time.RFC3339),
				)
				if err := alerter.SendAlert(ctx, title, text); err != nil {
					log.Printf("[dlq_monitor] failed to send alert for bucket %s: %v", bucket, err)
				}
			}
		}
	}
}
