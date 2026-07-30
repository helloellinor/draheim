¡# Dra heim - Go Application Deployment Dashboard

.PHONY: build run clean test server client

# Build both server and client
build: server client

# Build the server
server:
	go build -o draheim cmd/server/main.go

# Build the client
client:
	go build -o draheim-client cmd/client/client.go

# Run the server
run: server
	./draheim

# Clean build artifacts
clean:
	rm -f draheim draheim-client draheim-config.json draheim-client-config.json

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
deploy: server
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

# Client commands
client-list: client
	./draheim-client -action=list

client-deploy: client
	./draheim-client -action=deploy -name=$(NAME) -repo=$(REPO) -branch=$(BRANCH) -port=$(PORT)

client-stop: client
	./draheim-client -action=stop -name=$(NAME)

client-update: client
	./draheim-client -action=update -name=$(NAME)

client-logs: client
	./draheim-client -action=logs -name=$(NAME)