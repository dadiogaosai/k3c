# k3c — local k3s clusters on Apple `container`

BINARY  := k3c
PREFIX  ?= /usr/local
GOBIN   := $(shell go env GOPATH)/bin
LDFLAGS := -s -w \
	-X k3c/version.Version=dev \
	-X k3c/version.GitCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
	-X k3c/version.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.DEFAULT_GOAL := help

.PHONY: help all build fmt vet check test clean install install-user uninstall

help: ## show this help
	@echo "k3c — local k3s clusters on Apple container"
	@echo
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  make %-14s %s\n", $$1, $$2}'
	@echo
	@echo "variables: PREFIX=$(PREFIX) (install prefix)"

all: check build ## vet + format check + build

build: ## build the k3c binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

fmt: ## format the Go sources
	gofmt -w .

vet: ## run go vet
	go vet ./...

test: ## run tests
	go test ./...

check: vet ## vet + fail on unformatted files
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed for: $$unformatted"; exit 1; \
	fi

clean: ## remove the built binary
	rm -f $(BINARY)

install: build ## install system-wide to $(PREFIX)/bin (sudo if needed)
	@if [ -w $(PREFIX)/bin ]; then \
		install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY); \
	else \
		echo "$(PREFIX)/bin is not writable, using sudo..."; \
		sudo install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY); \
	fi
	@echo "installed: $(PREFIX)/bin/$(BINARY)"

install-user: build ## install to GOPATH/bin (no sudo; ensure it is on PATH)
	install -m 0755 $(BINARY) $(GOBIN)/$(BINARY)
	@echo "installed: $(GOBIN)/$(BINARY)"

uninstall: ## remove installed binaries
	rm -f $(GOBIN)/$(BINARY) 2>/dev/null || true
	@if [ -e $(PREFIX)/bin/$(BINARY) ]; then \
		rm -f $(PREFIX)/bin/$(BINARY) 2>/dev/null || sudo rm -f $(PREFIX)/bin/$(BINARY); \
	fi
