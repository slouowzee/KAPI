.PHONY: help build test lint clean check

APP_NAME := kapi

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the KAPI binary
	go build -o $(APP_NAME) main.go

test: ## Run unit tests with race detector
	go test -v -race ./...

lint: ## Run golangci-lint (make sure it is installed)
	golangci-lint run ./...

clean: ## Remove built binary and temporary files
	rm -f $(APP_NAME)
	rm -rf dist/

check: lint test build ## Run linters, tests, and build (great before a PR)

demo: build ## Record the VHS demo (make sure 'vhs' is installed)
	vhs < demo.tape
