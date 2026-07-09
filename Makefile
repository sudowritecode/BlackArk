.PHONY: build test release release-bundle migrate-up run-control smoke-cli-test

VERSION ?= dev
GOOS ?= linux
GOARCH ?= amd64
BINS := blackark blackark-control blackark-agent

LDFLAGS := -ldflags="-X github.com/sudowritecode/BlackArk/internal/control.Version=$(VERSION)"

build:
	mkdir -p bin
	go build $(LDFLAGS) -o bin/blackark ./cmd/blackark
	go build $(LDFLAGS) -o bin/blackark-control ./cmd/blackark-control
	go build $(LDFLAGS) -o bin/blackark-agent ./cmd/blackark-agent

test:
	go test ./...

release:
	rm -rf .artifacts
	mkdir -p .artifacts
	for bin in $(BINS); do \
		GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o ".artifacts/$$bin" "./cmd/$$bin"; \
		tar -C .artifacts -czf ".artifacts/$$bin-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz" "$$bin"; \
		rm ".artifacts/$$bin"; \
	done
	mkdir -p .artifacts/blackark-$(VERSION)-$(GOOS)-$(GOARCH)
	for bin in $(BINS); do \
		GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o ".artifacts/blackark-$(VERSION)-$(GOOS)-$(GOARCH)/$$bin" "./cmd/$$bin"; \
	done
	cp scripts/install.sh .artifacts/blackark-$(VERSION)-$(GOOS)-$(GOARCH)/install.sh
	tar -C .artifacts -czf ".artifacts/blackark-bundle-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz" "blackark-$(VERSION)-$(GOOS)-$(GOARCH)"
	cp ".artifacts/blackark-bundle-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz" ".artifacts/blackark-bundle-$(GOOS)-$(GOARCH).tar.gz"

release-bundle:
	rm -rf .artifacts
	mkdir -p .artifacts/blackark-$(VERSION)-$(GOOS)-$(GOARCH)
	for bin in $(BINS); do \
		GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o ".artifacts/blackark-$(VERSION)-$(GOOS)-$(GOARCH)/$$bin" "./cmd/$$bin"; \
	done
	cp scripts/install.sh .artifacts/blackark-$(VERSION)-$(GOOS)-$(GOARCH)/install.sh
	tar -C .artifacts -czf ".artifacts/blackark-bundle-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz" "blackark-$(VERSION)-$(GOOS)-$(GOARCH)"
	cp ".artifacts/blackark-bundle-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz" ".artifacts/blackark-bundle-$(GOOS)-$(GOARCH).tar.gz"

migrate-up:
	go run ./cmd/blackark-control migrate

run-control:
	go run ./cmd/blackark-control serve

smoke-cli-test: build
	BLACKARK_BIN=./bin/blackark scripts/smoke-cli.sh
