# Targets mirror the CI jobs in .github/workflows/ci.yml.

REDIS_ADDR ?= 127.0.0.1:6379

.PHONY: all lint test test-redis cover build

all: lint test

lint:
	go mod tidy -diff
	golangci-lint run ./...

# Unit tests plus client tests against an in-process miniredis.
test:
	go test -race -count=1 ./...

# Client tests against a real server. Uses DB 15 and flushes it.
test-redis:
	LAZYREDIS_TEST_REDIS_ADDR=$(REDIS_ADDR) go test -race -count=1 ./pkg/redis/

cover:
	go test -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

build:
	goreleaser build --snapshot --clean
