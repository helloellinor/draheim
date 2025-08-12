# Draheim Demo System

This directory contains a comprehensive demonstration of the Draheim deployment system, showcasing its capabilities for deploying and managing Go applications.

## Quick Start

Run the interactive demo script:

```bash
cd /path/to/draheim
./demo/demo.sh
```

This will automatically:
1. Build the Draheim system
2. Start the server
3. Deploy three sample applications
4. Provide an interactive menu for testing features

## Demo Applications

### 1. Hello World App (Port 8081)
A simple web application that displays a welcome message and demonstrates basic HTTP server deployment.

**Features:**
- Responsive web interface
- Health check endpoint
- Real-time clock display
- Environment variable usage

**Endpoints:**
- `GET /` - Main page with welcome message
- `GET /health` - Health check (JSON response)

### 2. API Server App (Port 8082)
A RESTful API server demonstrating stateful application deployment with task management.

**Features:**
- JSON API responses
- In-memory task storage
- Multiple endpoints
- Timestamp tracking

**Endpoints:**
- `GET /` - API information
- `GET /tasks` - Task list management
- `GET /health` - Service health status

### 3. Counter App (Port 8083)
An interactive counter application demonstrating stateful operations and user interaction.

**Features:**
- Interactive web interface
- State persistence during runtime
- POST endpoint handling
- Real-time updates

**Endpoints:**
- `GET /` - Interactive counter interface
- `POST /increment` - Increment counter
- `POST /reset` - Reset counter to zero
- `GET /api/count` - Get current count (JSON)
- `GET /health` - Health status

## Demo Features Showcased

### 🚀 Deployment Capabilities
- **Binary Deployment**: Deploy pre-built Go binaries
- **Git Integration**: Deploy directly from Git repositories
- **Multi-application**: Run multiple apps simultaneously
- **Port Management**: Automatic port assignment and management

### 🖥️ Management Features
- **Web Dashboard**: Full-featured web interface
- **Command Line**: Complete CLI client for remote management
- **Process Management**: tmux-based process isolation
- **Real-time Monitoring**: Live status updates via HTMX

### 📊 Monitoring & Logging
- **Live Logs**: Real-time log viewing through web interface
- **Health Checks**: Built-in health monitoring endpoints
- **Status Tracking**: Application state management
- **Performance Monitoring**: Basic stress testing capabilities

### ⚙️ Configuration
- **JSON Configuration**: Flexible configuration management
- **SSH Key Support**: Private repository access
- **Environment Variables**: Runtime configuration injection
- **Default Settings**: Sensible defaults with customization options

## Manual Testing

### Using the Web Interface

1. **Start the server:**
   ```bash
   make run
   ```

2. **Open the dashboard:**
   Visit http://localhost:8080

3. **Deploy applications:**
   - Use the deployment form to deploy new applications
   - Specify Git repositories or binary paths
   - Configure ports and branches

### Using the Command Line Client

1. **List applications:**
   ```bash
   ./draheim-client -action=list
   ```

2. **Deploy from local binary:**
   ```bash
   ./draheim-client -action=deploy -name=myapp -path=/path/to/binary -port=8084
   ```

3. **Deploy from Git repository:**
   ```bash
   ./draheim-client -action=deploy -name=gitapp -repo=https://github.com/user/repo.git -branch=main -port=8085
   ```

4. **Stop an application:**
   ```bash
   ./draheim-client -action=stop -name=myapp
   ```

5. **Update Git-based application:**
   ```bash
   ./draheim-client -action=update -name=gitapp
   ```

6. **View logs:**
   ```bash
   ./draheim-client -action=logs -name=myapp
   ```

## Testing Scenarios

### Scenario 1: Basic Deployment
Test deploying the demo applications individually:

```bash
# Build demo apps
cd demo/sample-apps/hello-world && go build . && cd ../../..

# Deploy via web interface
# Visit http://localhost:8080 and use the deployment form

# Test the deployment
curl http://localhost:8081/health
```

### Scenario 2: Git-based Deployment
Test deploying applications from Git repositories:

```bash
# Deploy from public repository
./draheim-client -action=deploy -name=example -repo=https://github.com/user/go-app.git -port=8090

# Update the application
./draheim-client -action=update -name=example
```

### Scenario 3: Multi-application Management
Deploy and manage multiple applications simultaneously:

```bash
# Deploy multiple apps
./demo/demo.sh

# Check all running apps
./draheim-client -action=list

# Test each app
curl http://localhost:8081/health
curl http://localhost:8082/tasks
curl http://localhost:8083/api/count
```

### Scenario 4: Error Handling
Test error scenarios and recovery:

```bash
# Try to deploy invalid application
./draheim-client -action=deploy -name=invalid -path=/nonexistent/path -port=8090

# Stop non-existent application
./draheim-client -action=stop -name=nonexistent

# Check error responses in web interface
```

## Performance Testing

### Load Testing
Test application performance under load:

```bash
# Install Apache Bench (if available)
apt-get install apache2-utils

# Test Hello World app
ab -n 1000 -c 10 http://localhost:8081/

# Test API Server
ab -n 1000 -c 10 http://localhost:8082/tasks

# Test Counter app
ab -n 100 -c 5 -p /dev/null -T application/x-www-form-urlencoded http://localhost:8083/increment
```

### Resource Monitoring
Monitor system resources during operation:

```bash
# Monitor tmux sessions
tmux list-sessions

# Monitor processes
ps aux | grep -E "(draheim|hello-world|api-server|counter-app)"

# Monitor ports
netstat -tlnp | grep -E "(8080|8081|8082|8083)"
```

## Troubleshooting

### Common Issues

1. **Port conflicts:**
   ```bash
   # Check what's using a port
   lsof -i :8080
   
   # Kill process using port
   kill $(lsof -t -i:8080)
   ```

2. **tmux session issues:**
   ```bash
   # List all sessions
   tmux list-sessions
   
   # Kill specific session
   tmux kill-session -t app-name
   
   # Kill all sessions
   tmux kill-server
   ```

3. **Build failures:**
   ```bash
   # Check Go version
   go version
   
   # Clean and rebuild
   make clean
   make build
   ```

### Debug Mode

Enable debug logging:

```bash
# Set debug environment
export DEBUG=1
./draheim

# Or run with verbose client
./draheim-client -action=list -v
```

## Integration with Production

### Deployment to Production Server

1. **Copy built binaries:**
   ```bash
   scp draheim user@server:/opt/draheim/
   scp draheim-client user@server:/opt/draheim/
   ```

2. **Set up as service:**
   ```bash
   # Create systemd service
   sudo cp demo/draheim.service /etc/systemd/system/
   sudo systemctl enable draheim
   sudo systemctl start draheim
   ```

3. **Configure reverse proxy:**
   ```nginx
   # Nginx configuration
   location / {
       proxy_pass http://localhost:8080;
       proxy_set_header Host $host;
       proxy_set_header X-Real-IP $remote_addr;
   }
   ```

### Security Considerations

- **SSH Keys**: Ensure proper SSH key permissions (600)
- **Firewall**: Configure firewall rules for application ports
- **Access Control**: Implement authentication for production use
- **SSL/TLS**: Use HTTPS in production environments

## Next Steps

After running the demo, you can:

1. **Explore the source code** in `cmd/server/` and `cmd/client/`
2. **Modify demo applications** in `demo/sample-apps/`
3. **Create your own applications** for deployment
4. **Extend the system** with additional features
5. **Deploy to production** following the integration guide

For more detailed information, see the main [README.md](../README.md) file.