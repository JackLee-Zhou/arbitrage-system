# contracts/ Makefile — 契约代码生成与校验
.PHONY: gen lint breaking clean

gen: ## 生成 Go + Python 代码
	buf generate

lint: ## 检查 proto 文件规范性
	buf lint

breaking: ## 检测是否有破坏性变更（需指定 base）
	buf breaking --against 'https://github.com/arbitrage-system/contracts.git#branch=main'

clean: ## 清理生成代码
	rm -rf generated/go/* generated/python/*
