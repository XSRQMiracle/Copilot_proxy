BINARY_NAME=copilot-proxy
GO_BUILD=CGO_ENABLED=0 go build -ldflags="-s -w"

.PHONY: all build frontend clean dist docker

all: build

frontend:
	cd web && npm i && npm run build

internal/web/dist: frontend
	rm -rf internal/web/dist
	mkdir -p internal/web/dist
	cp -r web/dist/. internal/web/dist/

build: internal/web/dist
	$(GO_BUILD) -o $(BINARY_NAME) ./cmd/copilot-proxy

build-windows: internal/web/dist
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o $(BINARY_NAME).exe ./cmd/copilot-proxy

build-linux-amd64: internal/web/dist
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o $(BINARY_NAME)-linux-amd64 ./cmd/copilot-proxy

build-linux-arm64: internal/web/dist
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o $(BINARY_NAME)-linux-arm64 ./cmd/copilot-proxy

build-darwin-amd64: internal/web/dist
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(BINARY_NAME)-darwin-amd64 ./cmd/copilot-proxy

build-darwin-arm64: internal/web/dist
	GOOS=darwin GOARCH=arm64 $(GO_BUILD) -o $(BINARY_NAME)-darwin-arm64 ./cmd/copilot-proxy

build-all: build-windows build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64

docker:
	docker build -t copilot-proxy .

dist: build-all
	mkdir -p dist
	tar -czf dist/$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64 config.example.json
	tar -czf dist/$(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64 config.example.json
	tar -czf dist/$(BINARY_NAME)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64 config.example.json
	tar -czf dist/$(BINARY_NAME)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64 config.example.json
	zip -j dist/$(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME).exe config.example.json

dev: frontend
	go run ./cmd/copilot-proxy

vet:
	go vet ./...

clean:
	rm -rf web/dist internal/web/dist $(BINARY_NAME) $(BINARY_NAME).exe $(BINARY_NAME)-* dist/
