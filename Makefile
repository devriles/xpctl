BINARY   := xpctl
MODULE   := github.com/devriles/xpctl
BIN_DIR  := bin
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X main.version=$(VERSION)"

.PHONY: build install lint test vet clean fmt setup-integration teardown-integration test-integration

build:
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) .

install:
	go install $(LDFLAGS) .

lint:
	golangci-lint run ./...

test:
	go test -race ./...

vet:
	go vet ./...

clean:
	rm -rf $(BIN_DIR)

fmt:
	gofmt -w .

setup-integration:
	bash hack/setup-kind.sh

teardown-integration:
	bash hack/teardown-kind.sh

test-integration:
	go test -race -tags integration -timeout 120s -count=1 -v ./internal/kube/; \
	status=$$?; \
	bash hack/teardown-kind.sh; \
	exit $$status
