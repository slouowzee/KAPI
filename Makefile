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
	@echo "Running tests..."
	@go test -v -race ./... > .test.log 2>&1; \
	EXIT_CODE=$$?; \
	if [ $$EXIT_CODE -ne 0 ]; then \
		grep -v "^=== RUN" .test.log | grep -v "\-\-\- PASS" | grep -v "^PASS$$" | grep -v "^ok\s" | grep -v "^?"; \
		echo ""; \
		printf "\033[31m{𐄂} Tests failed!\033[0m\n"; \
		rm -f .test.log; \
		exit 1; \
	fi; \
	PASSED=$$(grep -c "\-\-\- PASS" .test.log); \
	printf "\033[32m{✓} Passed $$PASSED tests successfully.\033[0m\n"; \
	rm -f .test.log

lint: ## Run golangci-lint (make sure it is installed)
	golangci-lint run ./...

clean: ## Remove built binary and temporary files
	rm -f $(APP_NAME)
	rm -rf dist/

check: lint test build ## Run linters, tests, and build (great before a PR)

demo: build ## Record the VHS demo (make sure 'vhs' is installed)
	vhs < demo.tape
