.PHONY: build install test clean

BUILD_DIR := bin

build:
	go build -o $(BUILD_DIR)/reminders ./cmd/reminders
	go build -o $(BUILD_DIR)/drafts ./cmd/drafts

install:
	go install ./cmd/reminders
	go install ./cmd/drafts

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
