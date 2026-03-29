BINARY=medscan
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_FLAGS=-ldflags="-s -w -X main.Version=$(VERSION)"
INSTALL_DIR=$(HOME)/.local/bin

.PHONY: build build-all clean install install-global install-system test

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

# Instalación sin root — copia a ~/.local/bin y actualiza el PATH
install-global: build
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@chmod +x $(INSTALL_DIR)/$(BINARY)
	@echo "medscan instalado en $(INSTALL_DIR)/$(BINARY)"
	@if ! grep -q '\.local/bin' $(HOME)/.bashrc 2>/dev/null; then \
		echo '\nexport PATH="$$HOME/.local/bin:$$PATH"' >> $(HOME)/.bashrc; \
		echo "PATH actualizado en ~/.bashrc"; \
	fi
	@if [ -f $(HOME)/.zshrc ] && ! grep -q '\.local/bin' $(HOME)/.zshrc 2>/dev/null; then \
		echo '\nexport PATH="$$HOME/.local/bin:$$PATH"' >> $(HOME)/.zshrc; \
		echo "PATH actualizado en ~/.zshrc"; \
	fi
	@echo ""
	@echo "Ejecuta:  source ~/.bashrc  (o ~/.zshrc)"
	@echo "Después:  medscan"

# Instalación con sudo — para /usr/local/bin (accesible por todos los usuarios)
install-system: build
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "medscan instalado en /usr/local/bin/medscan"

# Alias cómodo (apunta a install-global por defecto)
install: install-global

clean:
	rm -f $(BINARY)
	rm -rf dist/

test:
	go test ./...

