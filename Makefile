.PHONY: test test-unit test-e2e run build lint

# Full automated suite (L1 + L2 + L3). Single entrypoint for ticket gates.
test:
	go test ./... -count=1 -timeout 120s

test-unit:
	go test ./internal/... -count=1

test-e2e:
	go test ./e2e/... -count=1 -timeout 120s

lint:
	go vet ./...
	staticcheck ./...
	ineffassign ./...
	deadcode -test ./...

build:
	go build -o bin/miner ./cmd/miner

# Dev: export MINER_PIN first. Listens on :8080 (all interfaces) by default.
# Phone on same LAN: http://<pc-ip>:8080
run: build
	./bin/miner
