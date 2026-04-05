.PHONY: build run clean test lint

BIN=agent-router
GO=go

build:
	$(GO) build -o $(BIN) ./cmd/agent-router

run: build
	./$(BIN)

clean:
	rm -f $(BIN)

test:
	$(GO) test -v ./...

lint:
	golangci-lint run

deps:
	$(GO) mod tidy
	$(GO) mod download
