.PHONY: build install test clean

BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/reminders ./cmd/reminders
	go build -o $(BUILD_DIR)/drafts ./cmd/drafts
	swiftc -O -o $(BUILD_DIR)/reminders-helper internal/reminderskit/helper.swift

install: build
	go install ./cmd/reminders
	go install ./cmd/drafts
	cp $(BUILD_DIR)/reminders-helper $(shell go env GOPATH)/bin/reminders-helper

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
