.PHONY: start seed generate generate-check build-cli test fmt vet check

start:
	go run ./cmd/server

N ?= 5
seed:
	./scripts/seed.sh $(N)

generate:
	./scripts/generate-clients.sh

generate-check:
	./scripts/generate-clients.sh --check

build-cli:
	go build -o bin/mdo ./cmd/mdo

test:
	go test ./... -shuffle=on

fmt:
	gofmt -w .

vet:
	go vet ./...

# Run before finishing any change.
check: generate-check fmt vet test
