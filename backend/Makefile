BINARY := bin/server
PKG    := ./cmd/server

.PHONY: build run test tidy fmt lint lint-fix clean

build:
	go build -o $(BINARY) $(PKG)

run:
	go run $(PKG)

test:
	go test -race -shuffle=on ./...

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

clean:
	rm -rf bin/
