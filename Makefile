.PHONY: lint build test tidy

export GO111MODULE=on

default: tidy lint test

tidy:
	go mod tidy

lint:
	golangci-lint run

build: tidy
	go build -o devctl-go

test:
	go test -v -cover ./...
