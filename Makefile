# Binary names
BOT_BINARY=tv-bot
CLIENT_BINARY=tv-client

# Build target
build:
	go build -o $(BOT_BINARY) ./cmd/tv-bot
	go build -o $(CLIENT_BINARY) ./cmd/tv-client
	@if [ "$$(uname)" = "Darwin" ]; then \
		echo "Signing binaries for macOS..."; \
		codesign -s - -f $(BOT_BINARY); \
		codesign -s - -f $(CLIENT_BINARY); \
	fi

# Run target
run: build
	./$(BOT_BINARY)

# Helper to run client via go run (bypasses macOS binary blocking)
run-client:
	go run ./cmd/tv-client/main.go $(ARGS)

# Clean target
clean:
	go clean
	rm -f $(BOT_BINARY) $(CLIENT_BINARY)

# Help target
help:
	@echo "Available targets:"
	@echo "  build   Build the project"
	@echo "  run     Build and run the project"
	@echo "  clean   Remove the binary"
	@echo "  help    Show this help message"

.PHONY: build run clean help
