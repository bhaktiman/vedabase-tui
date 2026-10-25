VERSION ?= dev

.PHONY: build test

build:
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o vedabase ./cmd/vedabase

test:
	go test ./...
