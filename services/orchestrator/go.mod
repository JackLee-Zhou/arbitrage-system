module github.com/arbitrage-system/orchestrator

go 1.26.4

require (
	github.com/arbitrage-system/pkg/redis v0.0.0
	github.com/arbitrage-system/pkg/registry v0.0.0
	google.golang.org/grpc v1.64.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/redis/go-redis/v9 v9.20.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)

replace (
	github.com/arbitrage-system/pkg/redis => ../../pkg/redis
	github.com/arbitrage-system/pkg/registry => ../../pkg/registry
)
