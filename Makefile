GO ?= $(shell if command -v go >/dev/null 2>&1; then command -v go; elif [ -x /usr/local/go/bin/go ]; then echo /usr/local/go/bin/go; else echo go; fi)
BINARY ?= METRICPlus
ROOT := $(CURDIR)
OUT ?= $(ROOT)/bin/$(BINARY)
VERSION ?= dev

.PHONY: all build test fmt vet clean

all: build

build:
	$(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(OUT) ./src/metricplus

test:
	$(GO) test ./src/...

fmt:
	$(GO) fmt ./src/...

vet:
	$(GO) vet ./src/...

clean:
	rm -rf $(ROOT)/bin
