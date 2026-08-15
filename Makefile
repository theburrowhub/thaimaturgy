# thAImaturgy Makefile
# Run 'make help' to see available targets

# Variables
BINARY_NAME := thaimaturgy
BINARY_DIR := bin
CMD_DIR := ./cmd/thaimaturgy
BOT_BINARY_NAME := thaimaturgy-bot
BOT_CMD_DIR := ./cmd/thaimaturgy-bot
SERVER_BINARY_NAME := thaimaturgy-server
SERVER_CMD_DIR := ./cmd/thaimaturgy-server
NOVEL_BINARY_NAME := thaimaturgy-novel
NOVEL_CMD_DIR := ./cmd/thaimaturgy-novel
RULESYSTEM_BINARY_NAME := rulesystem-gen
RULESYSTEM_CMD_DIR := ./cmd/rulesystem-gen
WORLDPACK_BINARY_NAME := worldpack-gen
WORLDPACK_CMD_DIR := ./cmd/worldpack-gen
PKG := github.com/theburrowhub/thaimaturgy

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOVET := $(GOCMD) vet
GOLINT := golangci-lint

# Build flags
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Platform detection
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Colors for terminal output
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

.PHONY: all build build-bot build-server build-novel build-rulesystem-gen build-worldpack-gen rulesystem-examples worldpack-examples run clean test test-verbose test-coverage lint fmt vet tidy deps help install uninstall example-module modules

# Adventure modules
EXAMPLES_DIR := examples/adventures
DIST_DIR := dist/modules

##@ General

all: clean build ## Build the project (default)

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(CYAN)Usage:$(RESET)\n  make $(GREEN)<target>$(RESET)\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(GREEN)%-18s$(RESET) %s\n", $$1, $$2 } \
		/^##@/ { printf "\n$(YELLOW)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

build: ## Build the binary
	@echo "$(CYAN)Building $(BINARY_NAME)...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(BINARY_NAME)$(RESET)"

run: build ## Build and run the application
	@echo "$(CYAN)Running $(BINARY_NAME)...$(RESET)"
	./$(BINARY_DIR)/$(BINARY_NAME)

build-bot: ## Build the Telegram bot binary
	@echo "$(CYAN)Building $(BOT_BINARY_NAME)...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BOT_BINARY_NAME) $(BOT_CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(BOT_BINARY_NAME)$(RESET)"

build-server: ## Build the HTTP server binary (#36)
	@echo "$(CYAN)Building $(SERVER_BINARY_NAME)...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(SERVER_BINARY_NAME) $(SERVER_CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(SERVER_BINARY_NAME)$(RESET)"

build-novel: ## Build the session-novelization console binary
	@echo "$(CYAN)Building $(NOVEL_BINARY_NAME)...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(NOVEL_BINARY_NAME) $(NOVEL_CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(NOVEL_BINARY_NAME)$(RESET)"
build-rulesystem-gen: ## Build the rulesystem pack generator CLI
	@echo "$(CYAN)Building $(RULESYSTEM_BINARY_NAME)...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(RULESYSTEM_BINARY_NAME) $(RULESYSTEM_CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(RULESYSTEM_BINARY_NAME)$(RESET)"

build-worldpack-gen: ## Build the worldpack catalog generator CLI
	@echo "$(CYAN)Building $(WORLDPACK_BINARY_NAME)...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(WORLDPACK_BINARY_NAME) $(WORLDPACK_CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(WORLDPACK_BINARY_NAME)$(RESET)"

worldpack-examples: build-worldpack-gen ## Generate example worldpack catalogs
	@echo "$(CYAN)Generating worldpack examples...$(RESET)"
	@mkdir -p examples/worlds
	$(BINARY_DIR)/$(WORLDPACK_BINARY_NAME) -all -out examples/worlds/
	@echo "$(GREEN)Examples in examples/worlds/$(RESET)"

rulesystem-examples: build-rulesystem-gen ## Generate example rulesystem packs
	@echo "$(CYAN)Generating rulesystem examples...$(RESET)"
	$(BINARY_DIR)/$(RULESYSTEM_BINARY_NAME) -all -out examples/rulesystems/
	@echo "$(GREEN)Examples in examples/rulesystems/$(RESET)"

dev: ## Run with go run (faster iteration)
	@echo "$(CYAN)Running in dev mode...$(RESET)"
	$(GOCMD) run $(CMD_DIR)

watch: ## Watch for changes and rebuild (requires entr)
	@command -v entr >/dev/null 2>&1 || { echo "$(RED)entr is required: brew install entr$(RESET)"; exit 1; }
	@echo "$(CYAN)Watching for changes...$(RESET)"
	find . -name '*.go' | entr -r make run

##@ Testing

test: ## Run tests
	@echo "$(CYAN)Running tests...$(RESET)"
	$(GOTEST) ./... -race

test-verbose: ## Run tests with verbose output
	@echo "$(CYAN)Running tests (verbose)...$(RESET)"
	$(GOTEST) ./... -v -race

test-coverage: ## Run tests with coverage report
	@echo "$(CYAN)Running tests with coverage...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	$(GOTEST) ./... -race -coverprofile=$(BINARY_DIR)/coverage.out -covermode=atomic
	$(GOCMD) tool cover -html=$(BINARY_DIR)/coverage.out -o $(BINARY_DIR)/coverage.html
	@echo "$(GREEN)Coverage report: $(BINARY_DIR)/coverage.html$(RESET)"
	$(GOCMD) tool cover -func=$(BINARY_DIR)/coverage.out | tail -1

test-short: ## Run short tests only
	@echo "$(CYAN)Running short tests...$(RESET)"
	$(GOTEST) ./... -short

bench: ## Run benchmarks
	@echo "$(CYAN)Running benchmarks...$(RESET)"
	$(GOTEST) ./... -bench=. -benchmem

##@ Code Quality

lint: ## Run linter (requires golangci-lint)
	@command -v $(GOLINT) >/dev/null 2>&1 || { echo "$(RED)golangci-lint is required: brew install golangci-lint$(RESET)"; exit 1; }
	@echo "$(CYAN)Running linter...$(RESET)"
	$(GOLINT) run ./...

fmt: ## Format code
	@echo "$(CYAN)Formatting code...$(RESET)"
	$(GOFMT) -s -w .
	@echo "$(GREEN)Code formatted$(RESET)"

fmt-check: ## Check if code is formatted
	@echo "$(CYAN)Checking code format...$(RESET)"
	@test -z "$$($(GOFMT) -l .)" || { echo "$(RED)Code is not formatted. Run 'make fmt'$(RESET)"; $(GOFMT) -l .; exit 1; }
	@echo "$(GREEN)Code format OK$(RESET)"

vet: ## Run go vet
	@echo "$(CYAN)Running go vet...$(RESET)"
	$(GOVET) ./...

check: fmt-check vet test ## Run all checks (format, vet, test)

##@ Dependencies

deps: ## Download dependencies
	@echo "$(CYAN)Downloading dependencies...$(RESET)"
	$(GOMOD) download
	@echo "$(GREEN)Dependencies downloaded$(RESET)"

tidy: ## Tidy go.mod
	@echo "$(CYAN)Tidying go.mod...$(RESET)"
	$(GOMOD) tidy
	@echo "$(GREEN)go.mod tidied$(RESET)"

update: ## Update dependencies
	@echo "$(CYAN)Updating dependencies...$(RESET)"
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "$(GREEN)Dependencies updated$(RESET)"

vendor: ## Vendor dependencies
	@echo "$(CYAN)Vendoring dependencies...$(RESET)"
	$(GOMOD) vendor
	@echo "$(GREEN)Dependencies vendored$(RESET)"

##@ Build & Release

clean: ## Clean build artifacts
	@echo "$(CYAN)Cleaning...$(RESET)"
	rm -rf $(BINARY_DIR)
	rm -f coverage.out
	@echo "$(GREEN)Cleaned$(RESET)"

install: build ## Install to GOPATH/bin
	@echo "$(CYAN)Installing $(BINARY_NAME)...$(RESET)"
	cp $(BINARY_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "$(GREEN)Installed to $(GOPATH)/bin/$(BINARY_NAME)$(RESET)"

uninstall: ## Uninstall from GOPATH/bin
	@echo "$(CYAN)Uninstalling $(BINARY_NAME)...$(RESET)"
	rm -f $(GOPATH)/bin/$(BINARY_NAME)
	@echo "$(GREEN)Uninstalled$(RESET)"

# Cross-platform release targets were removed: the app (cmd/thaimaturgy) is now a
# Fyne GUI that needs CGO + per-OS GL/X11 toolchains and can't be cross-built with
# plain `go build`. A multiplatform GUI release (fyne-cross / per-OS runners) is a
# follow-up; for now build natively with `make build`. The bot cross-compiles
# (pure Go) if ever needed: GOOS=… GOARCH=… go build ./cmd/thaimaturgy-bot.

##@ Docker

docker-build: ## Build Docker image
	@echo "$(CYAN)Building Docker image...$(RESET)"
	docker build -t $(BINARY_NAME):$(VERSION) .
	@echo "$(GREEN)Docker image built: $(BINARY_NAME):$(VERSION)$(RESET)"

docker-run: docker-build ## Run in Docker
	@echo "$(CYAN)Running in Docker...$(RESET)"
	docker run -it --rm $(BINARY_NAME):$(VERSION)

##@ Adventure Modules

example-module: ## Package the example adventure (the-sunken-crypt) into a .tar.gz
	@echo "$(CYAN)Packaging example module...$(RESET)"
	@mkdir -p $(DIST_DIR)
	tar -czf $(DIST_DIR)/the-sunken-crypt.tar.gz -C $(EXAMPLES_DIR)/the-sunken-crypt .
	@echo "$(GREEN)Built: $(DIST_DIR)/the-sunken-crypt.tar.gz$(RESET)"

modules: ## Package every example adventure into dist/modules
	@echo "$(CYAN)Packaging all adventure modules...$(RESET)"
	@mkdir -p $(DIST_DIR)
	@for d in $(EXAMPLES_DIR)/*/; do \
		name=$$(basename $$d); \
		tar -czf $(DIST_DIR)/$$name.tar.gz -C $$d .; \
		echo "$(GREEN)Built: $(DIST_DIR)/$$name.tar.gz$(RESET)"; \
	done

##@ Info

version: ## Show version info
	@echo "$(CYAN)Version:$(RESET)    $(VERSION)"
	@echo "$(CYAN)Commit:$(RESET)     $(COMMIT)"
	@echo "$(CYAN)Build Time:$(RESET) $(BUILD_TIME)"
	@echo "$(CYAN)Go Version:$(RESET) $(shell go version)"

info: ## Show project info
	@echo "$(CYAN)Binary:$(RESET)  $(BINARY_NAME)"
	@echo "$(CYAN)Package:$(RESET) $(PKG)"
	@echo "$(CYAN)OS:$(RESET)      $(GOOS)"
	@echo "$(CYAN)Arch:$(RESET)    $(GOARCH)"
	@echo ""
	@make version

loc: ## Count lines of code
	@echo "$(CYAN)Lines of code:$(RESET)"
	@find . -name '*.go' -not -path './vendor/*' | xargs wc -l | tail -1

tree: ## Show project structure
	@command -v tree >/dev/null 2>&1 || { echo "$(RED)tree is required: brew install tree$(RESET)"; exit 1; }
	tree -I 'vendor|bin|.git' --dirsfirst
