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

.PHONY: build run test install uninstall clean

build:
	go build -o $(BINARY) .

run:
	go run . $(ARGS)

test:
	go test ./...

install:
	go install .
	@echo "installed -> $$(go env GOPATH)/bin/$(BINARY)  (ensure that dir precedes your node bin on PATH)"

uninstall:
	rm -f "$$(go env GOPATH)/bin/$(BINARY)"

clean:
	rm -f $(BINARY)
