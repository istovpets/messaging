GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

.PHONY: all test lint

all: test lint fix vet

test:
	go test -v -race ./...

fix:
	go fix ./...

vet:
	go vet ./...

lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) run ./... && echo "✅ No issues found."

$(GOLANGCI_LINT):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
