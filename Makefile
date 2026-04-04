GO ?= go

.PHONY: test build fmt start stop status

test:
	$(GO) test ./...

build:
	$(GO) build ./cmd/relayd
	$(GO) build ./cmd/relay-wrapper
	$(GO) build ./cmd/relay-install

fmt:
	gofmt -w $$(find cmd internal testkit -name '*.go' | sort)

start:
	./install.sh start

stop:
	./install.sh stop

status:
	./install.sh status
