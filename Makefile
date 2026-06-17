# Local development for the flmnt CLI — iterate without cutting a release.
#
#   make run ARGS="login --server-url https://mcp.staging.flmnt.dev --client-id <id>"
#   make run ARGS="workspace list"
#   make build && ./flmnt version
#   make test
#
# To make `flmnt` on your PATH (and the .mcp.json headersHelper) use the dev build:
#   make install      # -> $(go env GOPATH)/bin/flmnt  (ensure ~/go/bin precedes your node bin on PATH)
#   make uninstall    # then the released npm/brew `flmnt` resolves again

BINARY := flmnt
ARGS ?=

.PHONY: build run test smoke install uninstall clean

build:
	go build -o $(BINARY) .

run:
	go run . $(ARGS)

# Tests run with CGO_ENABLED=0 to MIRROR the release build (GoReleaser uses CGO=0).
# This catches CGO-stripped bugs (e.g. the macOS Keychain backend is compiled out)
# that a default `go test`/`go run` (CGO=1) would silently hide.
test:
	CGO_ENABLED=0 go test ./...

# Smoke-test the CGO-free binary the way it actually ships: it must run keychain-
# touching commands without "backend not available". Same check runs in release CI.
smoke:
	@tmp=$$(mktemp -d); \
	CGO_ENABLED=0 go build -o $$tmp/flmnt . && \
	$$tmp/flmnt version && \
	out=$$($$tmp/flmnt whoami --server-url https://smoke.invalid 2>&1 || true); \
	echo "$$out"; \
	rm -rf $$tmp; \
	echo "$$out" | grep -qiE 'keyring|backend not available' && { echo "FAIL: keychain backend unavailable"; exit 1; } || echo "smoke ok"

install:
	go install .
	@echo "installed -> $$(go env GOPATH)/bin/$(BINARY)  (ensure that dir precedes your node bin on PATH)"

uninstall:
	rm -f "$$(go env GOPATH)/bin/$(BINARY)"

clean:
	rm -f $(BINARY)
