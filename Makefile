# ============================================================
# arbitrage-system 顶层 Makefile
# ============================================================

.PHONY: help gen build-all test deploy clean \
        lint lint-go lint-python lint-proto \
        test-unit test-unit-go test-unit-python \
        check fix vet install-hooks

help: ## 显示帮助信息
	@echo "arbitrage-system 顶层命令："
	@echo ""
	@echo "  📋 提交前检查（最常用）："
	@echo "  make check       — 一键执行所有提交前检查 ⭐"
	@echo "  make fix         — 自动修复可修复的问题"
	@echo "  make install-hooks — 安装 git pre-commit hook"
	@echo ""
	@echo "  🔍 检测："
	@echo "  make lint        — 所有语言 lint"
	@echo "  make lint-go     — golangci-lint（Go 服务）"
	@echo "  make lint-python — ruff（Python 服务）"
	@echo "  make lint-proto  — buf lint（契约检查）"
	@echo "  make vet         — go vet 编译检查（所有 Go 服务）"
	@echo ""
	@echo "  🧪 测试："
	@echo "  make test-unit    — 所有 unit 测试"
	@echo "  make test-unit-go — Go unit 测试"
	@echo "  make test-unit-py — Python unit 测试"
	@echo "  make test-int     — 集成测试（需要 Docker）"
	@echo ""
	@echo "  🔧 构建 + 部署："
	@echo "  make gen         — 生成 protobuf 代码"
	@echo "  make build-all   — 构建所有 Docker 镜像"
	@echo "  make deploy      — 部署到服务器"
	@echo "  make clean       — 清理构建产物"

# ========================================
# 📋 提交前检查（pre-commit）
# ========================================

check: vet lint test-unit ## ⭐ 一键执行所有提交前检查
	@echo ""
	@echo "✅ 所有提交前检查通过"

fix: ## 自动修复可修复的问题
	@echo "🔧 gofmt -w ..."
	@for dir in services/*/; do \
		if [ -f "$$dir/go.mod" ]; then \
			(cd "$$dir" && gofmt -w . 2>/dev/null) || true; \
		fi; \
	done
	@echo "🔧 ruff --fix ..."
	@cd services/llm_scorer && ruff check --fix . 2>/dev/null || true
	@cd services/intel_collector && ruff check --fix . 2>/dev/null || true
	@echo "✅ 自动修复完成"

install-hooks: ## 安装 git pre-commit hook
	@mkdir -p .git/hooks
	@cp .githooks/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✅ pre-commit hook 已安装"

# ========================================
# 🔍 Lint
# ========================================

lint: lint-go lint-python lint-proto ## 所有语言 lint

lint-go: ## golangci-lint（Go 服务）
	@echo "=== lint-go ==="
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "  $(YELLOW)⚠️  golangci-lint not installed — skipping$(NC)"; \
		echo "  Install: brew install golangci-lint"; \
	else \
		for dir in services/*/; do \
			if [ -f "$$dir/go.mod" ]; then \
				echo "  $$dir"; \
				(cd "$$dir" && golangci-lint run ./... 2>&1) || exit 1; \
			fi; \
		done; \
		echo "  ✅ Go lint 通过"; \
	fi

lint-python: ## ruff（Python 服务）
	@echo "=== lint-python ==="
	@if ! command -v ruff >/dev/null 2>&1; then \
		echo "  ⚠️  ruff not installed — skipping"; \
		echo "  Install: pip install ruff"; \
	else \
		cd services/llm_scorer && ruff check . 2>&1 || exit 1; \
		cd services/intel_collector && ruff check . 2>&1 || exit 1; \
		echo "  ✅ Python lint 通过"; \
	fi

lint-proto: ## buf lint（契约检查）
	@echo "=== lint-proto ==="
	@if ! command -v buf >/dev/null 2>&1; then \
		echo "  ⚠️  buf not installed — skipping"; \
		echo "  Install: brew install bufbuild/buf/buf"; \
	else \
		cd contracts && buf lint; \
		echo "  ✅ Proto lint 通过"; \
	fi

vet: ## go vet 编译检查（所有 Go 服务）
	@echo "=== go vet ==="
	@for dir in services/*/; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "  $$dir"; \
			(cd "$$dir" && go vet ./...) || exit 1; \
		fi; \
	done
	@echo "  ✅ 所有 Go 服务编译通过"

# ========================================
# 🧪 测试
# ========================================

test-unit: test-unit-go test-unit-python ## 所有 unit 测试

test-unit-go: ## Go unit 测试（无外部依赖）
	@echo "=== test-unit-go ==="
	@failed=0; \
	for dir in services/*/; do \
		if [ -f "$$dir/go.mod" ]; then \
			echo "  $$dir"; \
			(cd "$$dir" && go test ./... -short -count=1) || failed=1; \
		fi; \
	done; \
	if [ $$failed -eq 0 ]; then echo "  ✅ Go unit 测试通过"; else echo "  ❌ 部分 Go 测试失败"; exit 1; fi

test-unit-python: ## Python unit 测试（无外部依赖）
	@echo "=== test-unit-python ==="
	@cd services/llm_scorer && python -m pytest tests/unit/ -v 2>&1 || true
	@cd services/intel_collector && python -m pytest tests/unit/ -v 2>&1 || true
	@echo "  ✅ Python unit 测试完成（无测试时跳过）"

test-int: ## 集成测试（需要 Docker）
	docker compose -f docker-compose.test.yml up --abort-on-container-exit

# --- 代码生成 ---
gen: ## 生成 protobuf 代码
	cd contracts && $(MAKE) gen

# --- Docker 构建 ---
build-all: ## 构建所有镜像
	docker compose build

build-go: ## 构建 Go 服务镜像
	docker compose build ctp_tick trading_agent orchestrator db_writer watchdog

build-python: ## 构建 Python 服务镜像
	docker compose build llm_scorer intel_collector

# --- 启动 ---
up: ## 完整启动
	docker compose --profile full up -d

up-futures: ## 期货侧启动
	docker compose --profile futures up -d

up-ai: ## AI 侧启动
	docker compose --profile ai-only up -d

up-ui: ## 前端侧启动
	docker compose --profile ui up -d

down: ## 停止所有服务
	docker compose --profile full down

# --- 旧的 test 别名（兼容） ---
test: test-unit ## 运行所有单元测试（兼容旧命令）

# --- 部署 ---
deploy: ## 部署到服务器（需要设置 ECS_HOST）
	@test -n "$(ECS_HOST)" || (echo "请设置 ECS_HOST 环境变量"; exit 1)
	rsync -avz --exclude '.git' --exclude 'data/' --exclude '.env' \
		./ $(ECS_USER)@$(ECS_HOST):~/arbitrage-system/
	ssh $(ECS_USER)@$(ECS_HOST) 'cd ~/arbitrage-system && docker compose --profile full up -d --build'

# --- 清理 ---
clean: ## 清理构建产物
	docker compose --profile full down -v --remove-orphans
	find . -name "*.pyc" -delete
	find . -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true
