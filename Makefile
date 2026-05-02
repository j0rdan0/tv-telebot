# Binary name
BINARY_NAME=tv-bot

# Build target
build:
	go build -o $(BINARY_NAME) ./cmd/tv-bot

# Run target
run: build
	./$(BINARY_NAME)

# Clean target
clean:
	go clean
	rm -f $(BINARY_NAME)

# Help target
help:
	@echo "Available targets:"
	@echo "  build   Build the project"
	@echo "  run     Build and run the project"
	@echo "  clean   Remove the binary"
	@echo "  help    Show this help message"

.PHONY: build run clean help
