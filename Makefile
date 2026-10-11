BIN := nworm
PREFIX ?= /usr/local

.PHONY: build install fmt test

build:
	go build -o $(BIN) .

install: build
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)

fmt:
	gofmt -w .

test:
	go test ./...
