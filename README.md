# Dra heim

A Go/HTMX application and dashboard for deployment and management of other Go applications to bitraf's heim server.

## Features

- **Web Dashboard**: Modern HTMX-powered interface for managing applications
- **Tmux Integration**: Uses tmux sessions for process management
- **Project Deployment**: Deploy and manage multiple Go applications
- **Real-time Monitoring**: Live status updates and log viewing
- **Simple Routing**: Protocol for routing between multiple projects
- **Git Repository Management**: Clone, build, and deploy directly from Git repositories
- **SSH Credential Support**: Secure SSH key handling for private repositories
- **Configuration Management**: JSON-based configuration system
- **Client/Server Architecture**: Separate client and server components
- **Auto-complete Forms**: Path suggestions and form auto-completion
- **Non-intrusive Updates**: HTMX-based polling that doesn't interfere with form filling

## Requirements

- Go 1.19+
- tmux
- Git (for repository-based deployments)
- Linux/Unix environment

## Quick Start

1. **Build both server and client:**
   ```bash
   make build
   ```

2. **Run the server:**
   ```bash
   make run
   ```

3. **Access the dashboard:**
   Open http://localhost:8080 in your browser

4. **Use the client (optional):**
   ```bash
   # List projects
   ./draheim-client -action=list
   
   # Deploy from Git repository
   ./draheim-client -action=deploy -name=myapp -repo=https://github.com/user/repo.git -branch=main -port=8081
   
   # Update a Git-based project
   ./draheim-client -action=update -name=myapp
   ```

## Usage

### Command Line

```bash
# Build both server and client
make build

# Build server only
make server

# Build client only  
make client

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

# Client commands
make client-list
make client-deploy NAME=myapp REPO=https://github.com/user/repo.git
make client-stop NAME=myapp
make client-update NAME=myapp
```

### Web Interface

The dashboard provides:
- **Project Management**: Deploy, start, stop, and monitor applications
- **Live Status**: Real-time updates on project health via HTMX polling
- **Log Viewing**: Access to application logs via tmux capture
- **Deployment Form**: Easy deployment of new Go applications
- **Git Integration**: Deploy directly from Git repositories with auto-build
- **Configuration Management**: View and manage server configuration
- **Path Auto-completion**: Suggested paths for common deployment directories

### Git Repository Deployment

You can deploy applications directly from Git repositories:

1. **Via Web Interface**: Fill in the Git Repository URL and branch in the deployment form
2. **Via Client**: Use the client with `-repo` and `-branch` parameters
3. **Automatic Building**: The system will clone the repo, build the Go application, and start it
4. **Updates**: Use the Update button or client command to pull latest changes and rebuild

### Project Structure

```
draheim/
├── cmd/
│   ├── server/
│   │   ├── main.go          # Main server application
│   │   └── main_test.go     # Server tests
│   └── client/
│       └── client.go        # Command-line client
├── Makefile                 # Build and deployment commands
├── README.md               # This file
└── .gitignore              # Git ignore rules
```

## Architecture

- **Frontend**: HTMX for dynamic updates without JavaScript complexity
- **Backend**: Go HTTP server with tmux process management
- **Process Management**: Each deployed application runs in its own tmux session
- **Routing**: Simple HTTP-based routing between projects
- **Git Integration**: Automatic cloning, building, and deployment from repositories
- **Configuration**: JSON-based configuration with sensible defaults
- **Client/Server**: Separate client application for remote management

## Configuration

The server uses a JSON configuration file (`draheim-config.json`) with the following options:

```json
{
  "server_port": ":8080",
  "repo_base_path": "/tmp/draheim-repos",
  "allowed_hosts": ["github.com", "gitlab.com", "bitbucket.org"],
  "ssh_key_path": "/home/user/.ssh/id_rsa", 
  "default_branch": "main"
}
```

The client uses a separate configuration file (`draheim-client-config.json`):

```json
{
  "server_url": "http://localhost:8080",
  "username": "user",
  "api_key": ""
}
```

Both configuration files are auto-generated with sensible defaults on first run.

## SSH Key Setup

For private Git repositories, place your SSH private key at the configured path (default: `~/.ssh/id_rsa`). The system will automatically use it for Git operations.

## Development

The application follows these principles:
- **Minimal Dependencies**: Only uses standard library + tmux + git
- **Simple Deployment**: Single binary with embedded templates
- **Tmux-First**: Leverages tmux for all process management
- **HTMX Integration**: Progressive enhancement for dynamic UI
- **Git-Native**: First-class support for Git-based deployments
- **Non-intrusive**: Form filling is never interrupted by auto-refresh

## Contributing

This project aims to define protocols for routing between multiple projects, deployment, beta testing and development on bitraf's heim server where only tmux and go are available.
