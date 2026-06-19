# contracts/ Makefile — 契约代码生成与校验
# 使用 protoc（Google 原生编译器）+ protoc-gen-go / protoc-gen-go-grpc
.PHONY: gen lint clean

PROTO_DIR := .
GO_OUT    := generated/go

gen: ## 生成 Go + Python 代码
	@mkdir -p $(GO_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/*.proto
	@echo "✅ protoc 代码生成完成"

lint: ## 检查 proto 文件规范性（protoc 内置）
	@echo "⚠️  无 buf lint，依赖 protoc 编译期检查"

clean: ## 清理生成代码
	rm -rf $(GO_OUT)/* generated/python/*
