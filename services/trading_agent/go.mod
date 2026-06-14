module github.com/arbitrage-system/trading_agent

go 1.26.4

require (
	github.com/arbitrage-system/pkg/redis v0.0.0
	github.com/arbitrage-system/pkg/registry v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/redis/go-redis/v9 v9.20.1 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	go.uber.org/atomic v1.11.0 // indirect
)

replace (
	github.com/arbitrage-system/pkg/redis => ../../pkg/redis
	github.com/arbitrage-system/pkg/registry => ../../pkg/registry
)

require github.com/arbitrage-system/pkg/config v0.0.0

replace github.com/arbitrage-system/pkg/config => ../../pkg/config
