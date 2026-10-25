VERSION ?= dev
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: build test release

build:
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o vedabase ./cmd/vedabase

test:
	go test ./...

# Cross-builds every platform into dist/ with checksums, ready for a GitHub
# release: make release VERSION=v0.1.0
release:
	rm -rf dist && mkdir -p dist
	for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; name=vedabase_$${os}_$${arch}; bin=vedabase; \
		[ $$os = windows ] && bin=vedabase.exe; \
		mkdir -p dist/$$name && \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION)" -o dist/$$name/$$bin ./cmd/vedabase && \
		if [ $$os = windows ]; then (cd dist/$$name && zip -q ../$$name.zip $$bin); \
		else tar -czf dist/$$name.tar.gz -C dist/$$name $$bin; fi && \
		rm -rf dist/$$name || exit 1; \
	done
	cp install.sh dist/
	cd dist && sha256sum vedabase_* > checksums.txt
