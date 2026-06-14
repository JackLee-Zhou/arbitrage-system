module github.com/arbitrage-system/db_writer

go 1.26.4

require (
	github.com/arbitrage-system/pkg/redis v0.0.0
	github.com/arbitrage-system/pkg/registry v0.0.0
	github.com/influxdata/influxdb-client-go/v2 v2.12.3
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/redis/go-redis/v9 v9.20.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.7.0 // indirect
)

require (
	github.com/deepmap/oapi-codegen v1.8.2 // indirect
	github.com/influxdata/line-protocol v0.0.0-20200327222509-2487e7298839 // indirect
)

replace (
	github.com/arbitrage-system/pkg/redis => ../../pkg/redis
	github.com/arbitrage-system/pkg/registry => ../../pkg/registry
)
