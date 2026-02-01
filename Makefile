.PHONY: build build-all clean run tidy

BINARY=adp-openai-gateway
DIST_DIR=dist
LDFLAGS=-ldflags="-s -w"

# 默认编译（当前平台）
build:
	go build -o $(BINARY) ./cmd/server

# 编译所有平台
build-all: clean-dist
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 ./cmd/server
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe ./cmd/server
	@echo "Build complete! Files in $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)/

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

clean-dist:
	rm -rf $(DIST_DIR)

clean-all: clean clean-dist

tidy:
	go mod tidy
