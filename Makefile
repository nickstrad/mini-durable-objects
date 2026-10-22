.PHONY: test fmt vet check

test:
	go test ./... -shuffle=on

fmt:
	gofmt -w .

vet:
	go vet ./...

# Run before finishing any change.
check: fmt vet test
