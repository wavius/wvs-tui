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

# Install the executable as wvs-tui
install: all
	@mkdir -p ~/.local/bin
	cp $(BUILD_DIR)/$(TARGET) ~/.local/bin/wvs
	@echo "Installed wvs to ~/.local/bin/wvs. Ensure ~/.local/bin is in your PATH."

.PHONY: all setup clean run install
