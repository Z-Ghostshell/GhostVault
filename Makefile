SHELL := /bin/bash
COMPOSE ?= docker compose

.PHONY: help setup up down logs ps build pull recreate clean test integration test-ingest-write build-bin gvsvd mcp ctl refresh-token rotate-token reset-vault-db dashboard-dev

# Local Vite dev server for dashboard (`dashboard/`). Override port if 5177 is taken.
DASHBOARD_DEV_PORT ?= 5177

# `make up` / `make build`: enable or disable optional services (case-insensitive).
# Examples: `make up` (defaults), `make up MCP=disable`, `make up Dashboard=disable`
# Compose profiles: `mcp` → gvmcp; `dashboard` → dashboard container (:80). Edge (:8989) proxies /dashboard/ to it. `make dashboard-dev` is separate (Vite :5177).
MCP ?= enable
Dashboard ?= enable

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?##"} {printf "  %-14s %s\n", $$1, $$2}'

setup: ## Create .env from .env.example if missing
	@test -f .env || cp .env.example .env
	@echo "Edit .env (OPENAI_API_KEY and OPENAI_BASE_URL if not api.openai.com). Then: make up"

up: setup ## Postgres + gvsvd + edge (:8989). MCP=enable|disable Dashboard=enable|disable (both default enable). BUILD=true rebuilds.
	@m_raw="$${MCP:-enable}"; [ -z "$$m_raw" ] && m_raw=enable; \
	m=$$(echo "$$m_raw" | tr '[:upper:]' '[:lower:]'); \
	d_raw="$${Dashboard:-enable}"; [ -z "$$d_raw" ] && d_raw=enable; \
	d=$$(echo "$$d_raw" | tr '[:upper:]' '[:lower:]'); \
	profiles=""; \
	if [ "$$m" = "enable" ]; then profiles="$$profiles --profile mcp"; fi; \
	if [ "$$d" = "enable" ]; then profiles="$$profiles --profile dashboard"; export WITH_DASHBOARD=1; else export WITH_DASHBOARD=0; fi; \
	if [ "$(BUILD)" = "true" ]; then $(COMPOSE) $$profiles up -d --build; else $(COMPOSE) $$profiles up -d; fi

down: ## Stop and remove containers (keeps volume)
	$(COMPOSE) down

logs: ## Follow gvsvd logs
	$(COMPOSE) logs -f ghostvault

ps: ## Compose status
	$(COMPOSE) ps

build: ## Build images only (same MCP= / Dashboard= as `up`)
	@m_raw="$${MCP:-enable}"; [ -z "$$m_raw" ] && m_raw=enable; \
	m=$$(echo "$$m_raw" | tr '[:upper:]' '[:lower:]'); \
	d_raw="$${Dashboard:-enable}"; [ -z "$$d_raw" ] && d_raw=enable; \
	d=$$(echo "$$d_raw" | tr '[:upper:]' '[:lower:]'); \
	profiles=""; \
	if [ "$$m" = "enable" ]; then profiles="$$profiles --profile mcp"; fi; \
	if [ "$$d" = "enable" ]; then profiles="$$profiles --profile dashboard"; export WITH_DASHBOARD=1; else export WITH_DASHBOARD=0; fi; \
	$(COMPOSE) $$profiles build

pull: ## Pull base images
	$(COMPOSE) pull

recreate: setup ## Rebuild images and force-recreate all stack services (MCP= / Dashboard= like `up`). postgres + ghostvault + edge + optional gvmcp/dashboard.
	@m_raw="$${MCP:-enable}"; [ -z "$$m_raw" ] && m_raw=enable; \
	m=$$(echo "$$m_raw" | tr '[:upper:]' '[:lower:]'); \
	d_raw="$${Dashboard:-enable}"; [ -z "$$d_raw" ] && d_raw=enable; \
	d=$$(echo "$$d_raw" | tr '[:upper:]' '[:lower:]'); \
	profiles=""; \
	if [ "$$m" = "enable" ]; then profiles="$$profiles --profile mcp"; fi; \
	if [ "$$d" = "enable" ]; then profiles="$$profiles --profile dashboard"; export WITH_DASHBOARD=1; else export WITH_DASHBOARD=0; fi; \
	$(COMPOSE) $$profiles up -d --build --force-recreate

clean: ## Remove containers and gv_pg volume (destructive)
	$(COMPOSE) down -v

reset-vault-db: ## Delete all vault data in Postgres (3× RESET confirm); then gvctl init
	@./scripts/reset-ghostvault-db.sh

test: ## Unit tests
	go test ./... -count=1

integration: ## Integration tests (Docker required)
	go test -tags=integration ./integration/... -count=1

test-ingest-write: ## End-to-end ingest → stats → retrieve (needs gvsvd + .ghostvault-bearer; rebuild gvsvd after store fixes)
	@./scripts/test/ingest-write-verify.sh

gvsvd: ## Build gvsvd server binary to bin/gvsvd
	@mkdir -p bin
	go build -o bin/gvsvd ./cmd/gvsvd

mcp: ## Build gvmcp MCP server binary to bin/gvmcp
	@mkdir -p bin
	go build -o bin/gvmcp ./cmd/gvmcp

ctl: ## Build gvctl REST helper binary to bin/gvctl
	@mkdir -p bin
	go build -o bin/gvctl ./cmd/gvctl

build-bin: gvsvd mcp ctl ## Build all cmd binaries to bin/

refresh-token: ctl ## Unlock and write .ghostvault-bearer for direnv (needs GHOSTVAULT_PASSWORD if encryption on)
	@./scripts/refresh-ghostvault-token.sh

rotate-token: refresh-token ## refresh-token + recreate gvmcp so the Docker MCP server reloads the new session (no-op if profile mcp isn't running)
	@unset GHOSTVAULT_BEARER_TOKEN; \
	if $(COMPOSE) ps --services --status running 2>/dev/null | grep -qx gvmcp; then \
		echo "recreating gvmcp so it reloads $${GHOSTVAULT_TOKEN_FILE:-.ghostvault-bearer}…"; \
		$(COMPOSE) --profile mcp up -d --force-recreate gvmcp; \
	else \
		echo "gvmcp not running (profile mcp disabled?) — token written but no container to recreate."; \
	fi

# Use `mise exec` when available so Node/npm come from mise.toml, not broken ~/.asdf shims ahead on PATH.
# Sources repo `.env` so `GHOSTVAULT_BASE_URL` (and optional `GHOSTVAULT_PROXY_URL`) apply to the Vite `/v1` proxy.
dashboard-dev: ## Local Vite dashboard (hot reload, no Docker). Needs gvsvd. Proxy: GHOSTVAULT_BASE_URL / GHOSTVAULT_PROXY_URL in `.env`, else http://127.0.0.1:8989/api or http://127.0.0.1:18080 if you publish gvsvd directly
	@set -a; [ -f .env ] && . ./.env; set +a; \
	cd dashboard && \
		if command -v mise >/dev/null 2>&1; then \
			mise exec -- npm install && \
			mise exec -- npm run dev -- --port $(DASHBOARD_DEV_PORT); \
		else \
			npm install && \
			npm run dev -- --port $(DASHBOARD_DEV_PORT); \
		fi
