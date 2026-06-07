# Version is derived from the git tag — `git describe` gives v0.1.0 at the tag
# and v0.1.0-3-gabc1234 three commits later. There is no version constant to
# hand-edit; `git tag vX.Y.Z` is the single source of truth. (Pattern: hindcast.)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: install build test vet version calque

# Install to $GOBIN/$GOPATH/bin with the version baked in.
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/calque

# Build a local ./calque binary with the version baked in.
build:
	go build -ldflags "$(LDFLAGS)" -o calque ./cmd/calque

test:
	go test ./...

vet:
	go vet ./...

# Print the version that a build would stamp.
version:
	@echo $(VERSION)

# Dogfood: run calque on its own source (calque must not carry the bug it detects).
calque: build
	./calque scan --left "**/*.go" --right "**/*.go" --min-score 0.12 || true
