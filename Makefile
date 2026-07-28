.PHONY: test test-unit test-e2e run build lint ocr-contract

# Full automated suite (L1 + L2 + L3). Single entrypoint for ticket gates.
# Does not require NDLOCR-Lite (uses ocr.Static).
test:
	go test ./... -count=1 -timeout 120s

test-unit:
	go test ./internal/... -count=1

test-e2e:
	go test ./e2e/... -count=1 -timeout 120s

# Real NDLOCR-Lite contract (needs MINER_NDL_ROOT / MINER_NDL_PYTHON / MINER_NDL_WORKER).
ocr-contract:
	MINER_OCR_CONTRACT=1 go test ./internal/adapters/ocr/ -run Contract -count=1 -timeout 15m -v

lint:
	go vet ./...
	staticcheck ./...
	ineffassign ./...
	deadcode -test ./...

build:
	go build -o bin/miner ./cmd/miner

# Dev: export MINER_PIN + MINER_NDL_* first. Listens on :8080 (all interfaces) by default.
# Phone on same LAN: http://<pc-ip>:8080
run: build
	./bin/miner
