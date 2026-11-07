.PHONY: build test vet fmt check integration clean
build:
	go build -trimpath -o bin/rbfqa ./cmd/rbfqa
test:
	go test -race ./...
vet:
	go vet ./...
fmt:
	gofmt -w cmd internal test examples
check: vet test build
	@test -z "$$(gofmt -l cmd internal test examples)"
integration:
	@test -n "$(RBFQA_BITCOIND)" || (echo 'Set RBFQA_BITCOIND to Bitcoin Core 32.1'; exit 2)
	go test -count=1 -v -timeout 15m ./test/integration
clean:
	rm -rf bin dist
