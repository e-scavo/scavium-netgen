APP_MODULE := scavium-netgen
BIN_DIR := bin
DIST_DIR := dist

CMDS := \
	scavium-netgen \
	wallet-new \
	wallet-balance \
	tx-send \
	tx-receipt \
	faucet-send \
	scavium-net-inspect \
	scavium-faucet

GO := go
GOFMT := gofmt
TARGET_OS ?= linux
TARGET_ARCH ?= amd64

.PHONY: all build clean fmt tidy test release release-linux-amd64 help

all: build

help:
	@echo "Available targets:"
	@echo "  make build              Build all binaries into ./bin"
	@echo "  make clean              Remove ./bin and ./dist"
	@echo "  make fmt                Run gofmt on all Go files"
	@echo "  make tidy               Run go mod tidy"
	@echo "  make test               Run go test ./..."
	@echo "  make release            Build release binaries into ./dist/<os>-<arch>"
	@echo "  make release-linux-amd64"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make release TARGET_OS=linux TARGET_ARCH=amd64"

build: $(BIN_DIR) $(addprefix $(BIN_DIR)/,$(CMDS))

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

$(BIN_DIR)/%:
	@echo "==> Building $*"
	$(GO) build -o $(BIN_DIR)/$* ./cmd/$*

fmt:
	@echo "==> Formatting Go files"
	@find . -type f -name '*.go' -not -path './bin/*' -not -path './dist/*' | xargs $(GOFMT) -w

tidy:
	@echo "==> Running go mod tidy"
	$(GO) mod tidy

test:
	@echo "==> Running tests"
	$(GO) test ./...

clean:
	@echo "==> Cleaning build artifacts"
	rm -rf $(BIN_DIR) $(DIST_DIR)

release: tidy test
	@echo "==> Building release for $(TARGET_OS)/$(TARGET_ARCH)"
	@mkdir -p $(DIST_DIR)/$(TARGET_OS)-$(TARGET_ARCH)
	@for cmd in $(CMDS); do \
		echo "==> Releasing $$cmd"; \
		GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) $(GO) build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/$(TARGET_OS)-$(TARGET_ARCH)/$$cmd ./cmd/$$cmd ; \
	done

release-linux-amd64:
	$(MAKE) release TARGET_OS=linux TARGET_ARCH=amd64