lint:
	golangci-lint run ./...

build:
	goreleaser build --snapshot --clean --single-target --skip before

.PHONY: test
test:
	go test ./...
