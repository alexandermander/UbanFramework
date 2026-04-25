APP := userver
BIN_DIR := bin
OUT := $(BIN_DIR)/$(APP)

.PHONY: all build run clean

all: build

build: $(OUT)

$(OUT): cmd/userver/main.go $(shell find internal -type f -name '*.go') go.mod go.sum
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(OUT) ./cmd/$(APP)

run: build
	./$(OUT) serve

clean:
	rm -rf $(BIN_DIR)
