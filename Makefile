PNPM ?= pnpm
WEB_DIR := web
DIST_DIR := internal/webui/dist
PLACEHOLDER := internal/webui/placeholder.html
BUILD_DIR := build
BINARY := $(BUILD_DIR)/web-reader

.PHONY: install dev-backend dev-frontend format lint test test-e2e build reset-dist clean

install:
	$(PNPM) --dir $(WEB_DIR) install --frozen-lockfile

dev-backend:
	go run ./cmd/web-reader

dev-frontend:
	$(PNPM) --dir $(WEB_DIR) dev

format:
	gofmt -w cmd internal
	$(PNPM) --dir $(WEB_DIR) format

lint:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	$(PNPM) --dir $(WEB_DIR) lint
	$(PNPM) --dir $(WEB_DIR) format:check

test:
	go test ./...
	$(PNPM) --dir $(WEB_DIR) test

test-e2e: build
	$(PNPM) --dir $(WEB_DIR) test:e2e

build:
	$(PNPM) --dir $(WEB_DIR) build
	@test -d $(DIST_DIR)/assets
	@! grep -q "frontend has not been built yet" $(DIST_DIR)/index.html
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/web-reader
	$(MAKE) reset-dist

reset-dist:
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)
	cp $(PLACEHOLDER) $(DIST_DIR)/index.html

clean:
	rm -rf $(BUILD_DIR)
	$(MAKE) reset-dist
