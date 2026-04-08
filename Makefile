GO ?= go

.PHONY: check test build fmt release-artifacts smoke-release-install release-track-version start stop status web-build

check:
	bash scripts/web/build-admin-ui.sh
	bash scripts/check/no-local-paths.sh
	bash scripts/check/no-legacy-names.sh
	files="$$(find cmd internal testkit -name '*.go' | sort)"; output="$$(gofmt -l $$files)"; test -z "$$output" || (echo "$$output" >&2; exit 1)
	bash scripts/check/release-track-version.sh
	bash scripts/check/smoke-install-release.sh
	$(GO) test ./...

test:
	$(GO) test ./...

build:
	bash scripts/web/build-admin-ui.sh
	$(GO) build ./cmd/codex-remote
	$(GO) build ./cmd/relayd
	$(GO) build ./cmd/relay-wrapper
	$(GO) build ./cmd/relay-install

web-build:
	bash scripts/web/build-admin-ui.sh

fmt:
	gofmt -w $$(find cmd internal testkit -name '*.go' | sort)

release-artifacts:
	@test -n "$(VERSION)" || (echo "VERSION is required, e.g. make release-artifacts VERSION=v0.1.0" >&2; exit 1)
	bash scripts/release/build-artifacts.sh "$(VERSION)"

smoke-release-install:
	bash scripts/check/smoke-install-release.sh

release-track-version:
	bash scripts/check/release-track-version.sh

start:
	./install.sh start

stop:
	./install.sh stop

status:
	./install.sh status
