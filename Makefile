.PHONY: build test vet clean run e2e install-macos uninstall-macos install-release sync-upstream help

# Deployboard's own version line, restarting at 0.0.1 — this is a fork, so the
# inherited upstream tags (v0.1.0-v0.3.1) must NOT leak into the binary's version.
# `git describe --tags` would resolve to v0.3.1, hence the explicit default.
# Override per build: make build VERSION=0.0.2
VERSION ?= 0.0.1

# Explicit package list, not ./... — landing/node_modules contains JavaScript
# packages that ship stray Go files, and Go's ./... would walk into them.
GO_PKGS := ./cmd/... ./internal/... ./web/...

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o deployboard ./cmd/deployboard

test:
	go test $(GO_PKGS) -count=1

vet:
	go vet $(GO_PKGS)

clean:
	rm -f deployboard

run: build
	./deployboard

e2e:
	npm run e2e

install-macos:
	bash ./install.sh

uninstall-macos:
	bash ./install.sh --uninstall

# Download the latest (or RELEASE_TAG=vX.Y.Z) GitHub Release binary and run install.sh.
# Example: make install-release RELEASE_TAG=v0.0.1
RELEASE_TAG ?=
install-release:
	bash ./install-release.sh $(if $(RELEASE_TAG),--from-release $(RELEASE_TAG),)

# Bring in upstream's work. Fetches, reports what is new, and stops — the rebase
# is deliberate, because conflict hot-spots are documented in docs/UPSTREAM.md.
sync-upstream:
	git fetch upstream
	@echo "== upstream commits not in this fork =="
	@git log --oneline HEAD..upstream/main || true
	@echo
	@echo "To take them: git rebase upstream/main && make vet test"

help:
	@echo "Available targets:"
	@echo "  build           - compile the binary (sets version from Makefile VERSION)"
	@echo "  test            - run all Go tests (count=1)"
	@echo "  vet             - go vet over the Go packages"
	@echo "  clean           - remove the binary"
	@echo "  run             - build and run the server"
	@echo "  e2e             - run Playwright end-to-end tests"
	@echo "  install-macos   - run install.sh (build from this checkout)"
	@echo "  install-release - install from GitHub Release (RELEASE_TAG=vX.Y.Z optional)"
	@echo "  uninstall-macos - run install.sh --uninstall"
	@echo "  sync-upstream   - fetch upstream and report new commits"
	@echo "  help            - show this help"
