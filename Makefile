BINARY    := lethe
VERSION   := 0.2.0
LDFLAGS   := -s -w
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build test test-integration vet cross-compile e2e clean

all: build

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/lethe

test:
	go test ./... -race

test-integration:
	go test ./internal/engine/ -tags=integration -race

vet:
	go vet ./...

e2e:
	./docker/e2e.sh

cross-compile:
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/lethe-$$os-$$arch ./cmd/lethe || exit 1; \
	done
	@echo "cross-compile done: dist/"

clean:
	rm -f $(BINARY)
	rm -rf dist/
