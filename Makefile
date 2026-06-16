# Version is derived from the git tag — `git describe` gives v0.1.0 at the tag
# and v0.1.0-3-gabc1234 three commits later. There is no version constant to
# hand-edit; `git tag vX.Y.Z` is the single source of truth. (Pattern: hindcast.)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: install build test vet version calque windows cross

# Install to $GOBIN/$GOPATH/bin with the version baked in.
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/calque

# Build a local ./calque binary with the version baked in.
build:
	go build -ldflags "$(LDFLAGS)" -o calque ./cmd/calque

# Cross-compile a Windows binary (bin/calque.exe) — copy it to the Windows box and
# run it from a repo checkout there. The single Go binary is self-contained; .rs
# scans build the embedded Rust helper on first run via the box's own cargo (and
# .py/.ts need python3/node on PATH — not needed for Rust-only repos like camber).
windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/calque.exe ./cmd/calque
	@echo "built bin/calque.exe ($(VERSION)) — needs cargo on the Windows box for .rs scans"

# Cross-compile all three desktop targets into bin/.
cross: windows
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/calque-linux-amd64   ./cmd/calque
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/calque-darwin-arm64  ./cmd/calque

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
