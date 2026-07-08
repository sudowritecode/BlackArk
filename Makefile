.PHONY: build test release migrate-up run-control smoke-cli-test

VERSION ?= dev
GOOS ?= linux
GOARCH ?= amd64
BINS := blackark blackark-control blackark-agent

build:
	mkdir -p bin
	go build -o bin/blackark ./cmd/blackark
	go build -o bin/blackark-control ./cmd/blackark-control
	go build -o bin/blackark-agent ./cmd/blackark-agent

test:
	go test ./...

release:
	rm -rf .artifacts
	mkdir -p .artifacts
	for bin in $(BINS); do \
		GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -o ".artifacts/$$bin" "./cmd/$$bin"; \
		tar -C .artifacts -czf ".artifacts/$$bin-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz" "$$bin"; \
		rm ".artifacts/$$bin"; \
	done

migrate-up:
	go run ./cmd/blackark-control migrate

run-control:
	go run ./cmd/blackark-control serve

smoke-cli-test: build
	BLACKARK_BIN=./bin/blackark scripts/smoke-cli.sh
