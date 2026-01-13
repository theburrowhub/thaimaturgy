# thAImaturgy Makefile
# Run 'make help' to see available targets

# Variables
BINARY_NAME := thaimaturgy
BINARY_DIR := bin
CMD_DIR := ./cmd/thaimaturgy
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

.PHONY: all build run clean test test-verbose test-coverage lint fmt vet tidy deps help install uninstall release

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

build-linux: ## Build for Linux
	@echo "$(CYAN)Building for Linux...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(BINARY_NAME)-linux-amd64$(RESET)"

build-darwin: ## Build for macOS
	@echo "$(CYAN)Building for macOS...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(BINARY_NAME)-darwin-{amd64,arm64}$(RESET)"

build-windows: ## Build for Windows
	@echo "$(CYAN)Building for Windows...$(RESET)"
	@mkdir -p $(BINARY_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(BINARY_NAME)-windows-amd64.exe$(RESET)"

release: clean build-linux build-darwin build-windows ## Build for all platforms
	@echo "$(GREEN)Release builds complete!$(RESET)"
	@ls -la $(BINARY_DIR)/

##@ Docker

docker-build: ## Build Docker image
	@echo "$(CYAN)Building Docker image...$(RESET)"
	docker build -t $(BINARY_NAME):$(VERSION) .
	@echo "$(GREEN)Docker image built: $(BINARY_NAME):$(VERSION)$(RESET)"

docker-run: docker-build ## Run in Docker
	@echo "$(CYAN)Running in Docker...$(RESET)"
	docker run -it --rm $(BINARY_NAME):$(VERSION)

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
