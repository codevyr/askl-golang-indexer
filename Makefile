BINARY_NAME := askl-golang-indexer
BUILD_DIR := bin

.PHONY: all build proto clean

all: build

build: proto
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/askl-golang-indexer

proto:
	protoc --go_out=. --go_opt=module=github.com/planetA/askl-golang-indexer proto/index.proto

clean:
	rm -rf $(BUILD_DIR)
