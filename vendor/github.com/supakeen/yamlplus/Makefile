.PHONY: test test-coverage lint fmt vet clean help

# Default target
.DEFAULT_GOAL := help

## test: Run all tests
test:
	go test -v -race ./...

## test-coverage: Run tests with coverage report
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out | grep total

## lint: Run all linters (staticcheck + go vet)
lint: staticcheck vet

## staticcheck: Run staticcheck
staticcheck:
	staticcheck ./...

## fmt: Format all Go files
fmt:
	gofmt -s -w .

## vet: Run go vet
vet:
	go vet ./...

## check: Run all checks (fmt, vet, staticcheck, test)
check: fmt vet staticcheck test

## clean: Remove build artifacts and coverage files
clean:
	rm -f coverage.out coverage.html
	go clean -cache -testcache

## help: Display this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
