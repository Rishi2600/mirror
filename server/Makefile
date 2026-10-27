# Settings come from the environment; a local .env, if present, is loaded too.
-include .env
export

VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)
BIN := bin/api

.PHONY: db-up db-down dev test gen gen-check fmt-check vet build check

## db-up: start the local databases (Docker Compose)
db-up: compose.yaml
	docker compose up -d --wait

## db-down: stop the local databases
db-down: compose.yaml
	docker compose down

compose.yaml:
	@echo "compose.yaml does not exist yet; the databases arrive in chunk 2" >&2
	@exit 1

## dev: run the API from source
dev:
	go run -ldflags "$(LDFLAGS)" ./cmd/api

## test: run all tests
test:
	go test ./...

## gen: regenerate code from api/openapi.yaml
gen:
	go generate ./...

## gen-check: fail if the generated code does not match the contract
gen-check:
	@tmp=$$(mktemp); trap 'rm -f "$$tmp"' EXIT; \
	go tool oapi-codegen -config api/oapi-codegen.yaml -o "$$tmp" api/openapi.yaml && \
	if ! cmp -s "$$tmp" api/api.gen.go; then \
		echo "api/api.gen.go does not match api/openapi.yaml; run make gen" >&2; exit 1; \
	fi

## fmt-check: fail if any Go file is not gofmt-formatted
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "these files need gofmt:" >&2; echo "$$unformatted" >&2; exit 1; \
	fi

vet:
	go vet ./...

## build: build the binary into bin/
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/api

## check: everything CI runs
check: fmt-check gen-check vet test build
