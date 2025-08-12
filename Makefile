# Dra heim - Go Application Deployment Dashboard

.PHONY: build run clean test

# Build the application
build:
	go build -o draheim main.go

# Run the application
run: build
	./draheim

# Clean build artifacts
clean:
	rm -f draheim

# Test the application
test:
	go test -v ./...

# Install dependencies
deps:
	go mod tidy

# Development mode with hot reload (requires air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Deploy to production (builds and starts with tmux)
deploy: build
	tmux new-session -d -s draheim ./draheim || tmux send-keys -t draheim C-c './draheim' Enter

# Stop the application
stop:
	tmux kill-session -t draheim 2>/dev/null || true

# Show status
status:
	@echo "Checking tmux sessions:"
	@tmux list-sessions 2>/dev/null || echo "No tmux sessions running"
	@echo "\nChecking processes:"
	@ps aux | grep draheim | grep -v grep || echo "No draheim processes found"

# Show logs
logs:
	tmux capture-pane -t draheim -p 2>/dev/null || echo "No draheim session found"