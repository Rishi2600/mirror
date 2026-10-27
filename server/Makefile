# Settings come from the environment; a local .env, if present, is loaded too.
-include .env
export

VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)
BIN := bin/api
DATA_DIR ?= ./data
# compose.yaml runs Postgres as this user.
HOST_UID := $(shell id -u)
HOST_GID := $(shell id -g)

.PHONY: db-up db-down dev test fmt-check vet build check

## db-up: start the local database (Docker Compose)
db-up:
	@# Create the folder first, or Docker creates it owned by root.
	mkdir -p $(DATA_DIR)/pg
	docker compose up -d --wait

## db-down: stop the local database (the data stays in DATA_DIR)
db-down:
	docker compose down

## dev: run the API from source
dev:
	go run -ldflags "$(LDFLAGS)" ./cmd/api

## test: run all tests
test:
	go test ./...

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
check: fmt-check vet test build
