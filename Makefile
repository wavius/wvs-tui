# Variables
-include .env
BUILD_DIR = build
TARGET = wvs-tui

# Default target to run the build
all: setup
	go build -ldflags="-X 'main/scraper.TMDBApiKey=$(API_KEY)'" -o $(BUILD_DIR)/$(TARGET)

# Target to create the directory
setup:
	mkdir -p $(BUILD_DIR)

# Clean up build artifacts
clean:
	rm -rf $(BUILD_DIR)

# Build and run the executable
run: all
	./$(BUILD_DIR)/$(TARGET)

.PHONY: all setup clean run
