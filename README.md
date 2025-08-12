# Dra heim

A Go/HTMX application and dashboard for deployment and management of other Go applications to bitraf's heim server.

## Features

- **Web Dashboard**: Modern HTMX-powered interface for managing applications
- **Tmux Integration**: Uses tmux sessions for process management
- **Project Deployment**: Deploy and manage multiple Go applications
- **Real-time Monitoring**: Live status updates and log viewing
- **Simple Routing**: Protocol for routing between multiple projects

## Requirements

- Go 1.19+
- tmux
- Linux/Unix environment

## Quick Start

1. **Build and run the dashboard:**
   ```bash
   make build
   make run
   ```

2. **Access the dashboard:**
   Open http://localhost:8080 in your browser

3. **Deploy a Go application:**
   - Use the web interface to specify project name, binary path, and port
   - The application will be started in a tmux session
   - Monitor status and view logs through the dashboard

## Usage

### Command Line

```bash
# Build the application
make build

# Run in development mode
make dev

# Deploy to production (runs in tmux)
make deploy

# Stop the application
make stop

# Check status
make status

# View logs
make logs
```

### Web Interface

The dashboard provides:
- **Project Management**: Deploy, start, stop, and monitor applications
- **Live Status**: Real-time updates on project health
- **Log Viewing**: Access to application logs via tmux capture
- **Deployment Form**: Easy deployment of new Go applications

### Project Structure

```
draheim/
├── main.go          # Main application with web server and tmux integration
├── Makefile         # Build and deployment commands
└── README.md        # This file
```

## Architecture

- **Frontend**: HTMX for dynamic updates without JavaScript complexity
- **Backend**: Go HTTP server with tmux process management
- **Process Management**: Each deployed application runs in its own tmux session
- **Routing**: Simple HTTP-based routing between projects

## Development

The application follows these principles:
- **Minimal Dependencies**: Only uses standard library + tmux
- **Simple Deployment**: Single binary with embedded templates
- **Tmux-First**: Leverages tmux for all process management
- **HTMX Integration**: Progressive enhancement for dynamic UI

## Contributing

This project aims to define protocols for routing between multiple projects, deployment, beta testing and development on bitraf's heim server where only tmux and go are available.
