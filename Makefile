.PHONY: build install test clean

BINARY := reminders
BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/reminders

install:
	go install ./cmd/reminders

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
