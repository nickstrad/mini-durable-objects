.PHONY: start seed events generate generate-check build build-server build-cli test race fmt vet check clean

# Extra server flags, for example: make start ARGS="-events data/events -log-level debug"
ARGS ?=

start:
	go run ./cmd/server $(ARGS)

N ?= 5
seed:
	./scripts/seed.sh $(N)

# DuckDB analysis of the event CSVs, for example: make events Q=contention DIR=data/events
DIR ?= data/events
Q ?=
events:
	./scripts/events.sh -d $(DIR) $(Q)

generate:
	./scripts/generate-clients.sh

generate-check:
	./scripts/generate-clients.sh --check

build: build-server build-cli

build-server:
	go build -o bin/server ./cmd/server

build-cli:
	go build -o bin/mdo ./cmd/mdo

test:
	go test ./... -shuffle=on

race:
	go test -race ./... -shuffle=on

fmt:
	gofmt -w .

vet:
	go vet ./...

# Run before finishing any change.
check: generate-check fmt vet test

clean:
	rm -rf bin
