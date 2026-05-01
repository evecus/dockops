.PHONY: all dev build build-web build-go clean run install

VERSION ?= dev
BINARY  := dockops

all: build

# ===== Development =====
dev:
	@echo "Starting dev servers..."
	@cd web && npm run dev &
	@go run . -c config.yaml

dev-frontend:
	cd web && npm install && npm run dev

dev-backend:
	go run . -c config.yaml

# ===== Build =====
build: build-web build-go

build-web:
	@echo "Building frontend..."
	cd web && npm install && npm run build

build-go:
	@echo "Building binary..."
	CGO_ENABLED=1 go build \
		-ldflags="-s -w -X main.Version=$(VERSION)" \
		-o $(BINARY) .
	@echo "Binary: ./$(BINARY)"

# Cross-compile targets
build-linux-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
	go build -ldflags="-s -w" -o dist/$(BINARY)-linux-amd64 .

build-linux-arm64:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
	CC=aarch64-linux-gnu-gcc \
	go build -ldflags="-s -w" -o dist/$(BINARY)-linux-arm64 .

# ===== Run =====
run: build
	./$(BINARY) -c config.yaml

# ===== Install =====
install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed to /usr/local/bin/$(BINARY)"

# ===== Clean =====
clean:
	rm -f $(BINARY)
	rm -rf dist/
	rm -rf web/dist/

# ===== Dependencies =====
deps:
	go mod tidy
	cd web && npm install

# ===== Lint =====
lint:
	go vet ./...
	cd web && npx eslint src/ --ext .vue,.js

# ===== Config template =====
config:
	@echo "http_port: 8080" > config.yaml
	@echo "https_port: 8443" >> config.yaml
	@echo "# cert_path: /path/to/cert.pem" >> config.yaml
	@echo "# key_path: /path/to/key.pem" >> config.yaml
	@echo "data_path: ./data" >> config.yaml
	@echo "Created config.yaml"
