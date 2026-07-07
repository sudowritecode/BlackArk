.PHONY: build test migrate-up run-control

build:
	mkdir -p bin
	go build -o bin/blackark ./cmd/blackark
	go build -o bin/blackark-control ./cmd/blackark-control
	go build -o bin/blackark-agent ./cmd/blackark-agent

test:
	go test ./...

migrate-up:
	go run ./cmd/blackark-control migrate

run-control:
	go run ./cmd/blackark-control serve
