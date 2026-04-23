.PHONY: all tidy build test lint run-mintburnd

all: tidy build test

tidy:
	go mod tidy

build:
	go build ./...

test:
	go test -race ./...

lint:
	go vet ./...

run-mintburnd:
	go run ./services/mint-burn/cmd/mintburnd
