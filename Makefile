SHELL := /bin/bash
BIN := bin/mcu-agent
.PHONY: build run clean fmt test

build:
	mkdir -p bin
	GO111MODULE=on go build -o $(BIN) ./cmd/mcu-agent

run: build
	$(BIN)

clean:
	rm -rf bin

fmt:
	gofmt -s -w .

test:
	go test ./...

lint:
	golangci-lint run --timeout 5m ./...

