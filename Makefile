.PHONY: test test-unit test-e2e run run-tunnel build lint ocr-install ocr-env ocr-contract

# GNU make (Linux + macOS Xcode/Homebrew). Repo-local NDLOCR-Lite install.
# Override with MINER_NDL_* env. Paths work on macOS (spaces in CURDIR quoted in recipes).
SHELL := /bin/bash
NDL_ROOT   ?= $(CURDIR)/.deps/ndlocr-lite
NDL_PYTHON ?= $(NDL_ROOT)/.venv/bin/python
NDL_WORKER ?= $(CURDIR)/scripts/ndl_ocr_worker.py

# Defaults only for run / ocr-contract / ocr-env (not exported globally —
# make test must stay free of a real OCR install).
OCR_ENV = \
	MINER_NDL_ROOT=$${MINER_NDL_ROOT:-$(NDL_ROOT)} \
	MINER_NDL_PYTHON=$${MINER_NDL_PYTHON:-$(NDL_PYTHON)} \
	MINER_NDL_WORKER=$${MINER_NDL_WORKER:-$(NDL_WORKER)}

# Full automated suite (L1 + L2 + L3). Single entrypoint for ticket gates.
# Does not require NDLOCR-Lite (uses ocr.Static).
test:
	go test ./... -count=1 -timeout 120s

test-unit:
	go test ./internal/... -count=1

test-e2e:
	go test ./e2e/... -count=1 -timeout 120s

# Clone NDLOCR-Lite + Python venv + requirements-ocr.txt into .deps/ (idempotent).
ocr-install:
	@chmod +x scripts/install_ndlocr.sh
	./scripts/install_ndlocr.sh

# Print export lines for the current install path (no install).
ocr-env:
	@echo "export MINER_NDL_ROOT='$${MINER_NDL_ROOT:-$(NDL_ROOT)}'"
	@echo "export MINER_NDL_PYTHON='$${MINER_NDL_PYTHON:-$(NDL_PYTHON)}'"
	@echo "export MINER_NDL_WORKER='$${MINER_NDL_WORKER:-$(NDL_WORKER)}'"

# Real NDLOCR-Lite contract (uses MINER_NDL_* or .deps defaults).
ocr-contract:
	@root="$${MINER_NDL_ROOT:-$(NDL_ROOT)}"; \
	py="$${MINER_NDL_PYTHON:-$(NDL_PYTHON)}"; \
	test -f "$$root/src/ocr.py" || (echo "OCR not installed. Run: make ocr-install"; exit 1); \
	test -f "$$py" || (echo "OCR venv missing. Run: make ocr-install"; exit 1); \
	$(OCR_ENV) MINER_OCR_CONTRACT=1 go test ./internal/adapters/ocr/ -run Contract -count=1 -timeout 15m -v

lint:
	go vet ./...
	staticcheck ./...
	ineffassign ./...
	deadcode -test ./...

build:
	go build -o bin/miner ./cmd/miner

# Shared preflight for run / run-tunnel (PIN via env or .env + OCR install).
define RUN_PREFLIGHT
	@if [ -z "$$MINER_PIN" ]; then \
		if [ ! -f .env ] || ! grep -qE '^[[:space:]]*(export[[:space:]]+)?MINER_PIN=.+' .env; then \
			echo "MINER_PIN is required (add to .env or export MINER_PIN=...)"; \
			echo "  cp .env.example .env   # then edit MINER_PIN"; \
			exit 1; \
		fi; \
	fi
	@root="$${MINER_NDL_ROOT:-$(NDL_ROOT)}"; \
	py="$${MINER_NDL_PYTHON:-$(NDL_PYTHON)}"; \
	worker="$${MINER_NDL_WORKER:-$(NDL_WORKER)}"; \
	test -f "$$root/src/ocr.py" || (echo "OCR not installed. Run: make ocr-install"; exit 1); \
	test -f "$$py" || (echo "OCR venv missing. Run: make ocr-install"; exit 1); \
	test -f "$$worker" || (echo "OCR worker missing: $$worker"; exit 1)
endef

# Dev: set MINER_PIN in .env (or export). OCR: make ocr-install (or set MINER_NDL_*).
# Listens on :8080 (all interfaces) by default. Phone on same LAN: http://<pc-ip>:8080
run: build
	$(RUN_PREFLIGHT)
	$(OCR_ENV) ./bin/miner

# HTTPS via free Cloudflare quick tunnel (trycloudflare.com). Needs cloudflared.
# Use for iPhone Safari camera (secure context). Public URL until Ctrl+C — PIN only.
# Default bind: 127.0.0.1:8080 (override with MINER_ADDR if you also want LAN HTTP).
run-tunnel: build
	$(RUN_PREFLIGHT)
	@command -v cloudflared >/dev/null 2>&1 || ( \
		echo "cloudflared not found. Install: brew install cloudflared"; \
		echo "  or https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"; \
		exit 1)
	@chmod +x scripts/run_tunnel.sh
	$(OCR_ENV) ./scripts/run_tunnel.sh
