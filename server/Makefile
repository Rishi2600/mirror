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

.PHONY: db-up db-down dev seed test fmt-check vet build check

## db-up: start Postgres and the embedding server (Docker Compose);
## SERVICES=pg starts only Postgres (CI does this)
db-up:
	@# Create the folders first, or Docker creates them owned by root.
	mkdir -p $(DATA_DIR)/pg $(DATA_DIR)/models
	docker compose up -d --wait $(SERVICES)

## db-down: stop them (the data stays in DATA_DIR)
db-down:
	docker compose down

## dev: run the API from source
dev:
	go run -ldflags "$(LDFLAGS)" ./cmd/api

## seed: fill the dev database with made-up candidates and hiring cards, and
## report how well the timelines match them (needs make db-up)
seed:
	go run ./cmd/seed

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
