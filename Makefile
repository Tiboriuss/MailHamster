BINARY  := mailhamster
MODULE  := github.com/Tiboriuss/MailHamster
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build release test lint clean

all: build

build:
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/mailhamster

release: dist/mailhamster-linux-amd64 dist/mailhamster-linux-arm64
	@echo "Binaries in dist/:"
	@ls -lh dist/

dist/mailhamster-linux-amd64:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $@ ./cmd/mailhamster

dist/mailhamster-linux-arm64:
	@mkdir -p dist
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $@ ./cmd/mailhamster

test:
	go test ./...

lint:
	go vet ./...
	@command -v golangci-lint &>/dev/null && golangci-lint run || true

clean:
	rm -rf bin/ dist/
