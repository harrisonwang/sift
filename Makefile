BINARY := sift
PKG    := ./cmd/sift
BIN    := bin/$(BINARY)

# Pure-Go static build (no CGO) — single binary, zero runtime deps.
export CGO_ENABLED := 0

.PHONY: build test vet fmt run tidy clean

build: ## Build the sift binary into ./bin
	go build -o $(BIN) $(PKG)

test: ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format the code
	gofmt -s -w .

tidy: ## Tidy go.mod/go.sum
	go mod tidy

run: build ## Build then run discover with the example config
	$(BIN) -c config.example.yaml discover

clean: ## Remove build output and local runtime data
	rm -rf bin dist tmp_run
