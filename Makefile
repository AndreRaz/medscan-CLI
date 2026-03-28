BINARY=medscan
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_FLAGS=-ldflags="-s -w -X main.Version=$(VERSION)"

.PHONY: build build-all clean install

build:
	go build $(BUILD_FLAGS) -o $(BINARY) .

build-all:
	@echo "Building for all platforms..."
	GOOS=linux   GOARCH=amd64  go build $(BUILD_FLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64  go build $(BUILD_FLAGS) -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64  go build $(BUILD_FLAGS) -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64  go build $(BUILD_FLAGS) -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64  go build $(BUILD_FLAGS) -o dist/$(BINARY)-windows-amd64.exe .
	@echo "Binarios en ./dist/"

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "✅ medscan instalado en /usr/local/bin/medscan"

clean:
	rm -f $(BINARY)
	rm -rf dist/

test:
	go test ./...
