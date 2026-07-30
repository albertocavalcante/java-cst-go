# java-cst-go development commands

default: test

build:
    go build ./...

generate:
    go generate ./ast

sync-grammar-kinds:
    go run ./cmd/syntaxgen -schema schema/java-syntax.json -sync-grammar-kinds
    go generate ./ast

test:
    go tool -modfile=tools.go.mod gotestsum --format pkgname-and-test-fails -- ./...

test-race:
    go tool -modfile=tools.go.mod gotestsum --format pkgname-and-test-fails -- -race ./...

lint:
    go tool -modfile=tools.go.mod golangci-lint run --timeout=5m

fmt:
    gofmt -w .

fmt-check:
    test -z "$(gofmt -l .)"

vet:
    go vet ./...

tidy:
    go mod tidy
    go mod tidy -modfile=tools.go.mod

tidy-check:
    go mod tidy
    go mod tidy -modfile=tools.go.mod
    git diff --exit-code -- go.mod go.sum tools.go.mod tools.go.sum

check: fmt-check vet lint test-race tidy-check
