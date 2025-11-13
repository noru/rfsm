.PHONY: help build test clean run fmt vet lint install demo

# Default target
.DEFAULT_GOAL := help

# Variables
BINARY_NAME := rfsm
MAIN_PKG := ./cmd/demo
SRC_PKG := ./...
TEST_PKG := ./...

# Colors for output
GREEN  := \033[0;32m
YELLOW := \033[0;33m
NC     := \033[0m # No Color

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'

build: ## Build the project
	@echo "$(YELLOW)Building...$(NC)"
	@go build -o $(BINARY_NAME) $(MAIN_PKG)
	@echo "$(GREEN)Build complete!$(NC)"

test: ## Run tests
	@echo "$(YELLOW)Running tests...$(NC)"
	@go test -v $(TEST_PKG)
	@echo "$(GREEN)Tests complete!$(NC)"

test-coverage: ## Run tests with coverage
	@echo "$(YELLOW)Running tests with coverage...$(NC)"
	@go test -v -coverprofile=coverage.out $(TEST_PKG)
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

run: ## Run the demo application
	@echo "$(YELLOW)Running demo...$(NC)"
	@go run $(MAIN_PKG)

fmt: ## Format code
	@echo "$(YELLOW)Formatting code...$(NC)"
	@go fmt $(SRC_PKG) $(MAIN_PKG)
	@echo "$(GREEN)Format complete!$(NC)"

vet: ## Run go vet
	@echo "$(YELLOW)Running go vet...$(NC)"
	@go vet $(SRC_PKG) $(MAIN_PKG)
	@echo "$(GREEN)Vet complete!$(NC)"

lint: vet ## Run linters (vet + fmt check)
	@echo "$(YELLOW)Checking formatting...$(NC)"
	@if [ $$(gofmt -l $(SRC_PKG) $(MAIN_PKG) | wc -l) -ne 0 ]; then \
		echo "$(YELLOW)Code is not formatted. Run 'make fmt'$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Lint complete!$(NC)"

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@go clean -cache
	@echo "$(GREEN)Clean complete!$(NC)"

install: ## Install dependencies
	@echo "$(YELLOW)Installing dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)Install complete!$(NC)"

demo: run ## Alias for run

all: clean fmt lint test build ## Run fmt, lint, test, and build

