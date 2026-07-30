#!/bin/bash

# Draheim Demo Script
# This script demonstrates the full capabilities of the Draheim deployment system

set -e

echo "🎬 =================================================="
echo "🎬  DRAHEIM DEPLOYMENT SYSTEM DEMO"
echo "🎬 =================================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_step() {
    echo -e "${BLUE}📍 Step $1:${NC} $2"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${CYAN}ℹ️  $1${NC}"
}

# Check if we're in the right directory
if [ ! -f "Makefile" ] || [ ! -d "cmd" ]; then
    print_error "Please run this script from the draheim root directory"
    exit 1
fi

# Step 1: Build the system
print_step "1" "Building Draheim server and client"
make clean
make build
print_success "Build completed successfully"
echo ""

# Step 2: Start the server in background
print_step "2" "Starting Draheim server"
./draheim &
SERVER_PID=$!
echo "Server started with PID: $SERVER_PID"

# Wait for server to start
sleep 3

# Check if server is running
if kill -0 $SERVER_PID 2>/dev/null; then
    print_success "Draheim server is running on http://localhost:8080"
else
    print_error "Failed to start Draheim server"
    exit 1
fi
echo ""

# Step 3: Test client connectivity
print_step "3" "Testing client connectivity"
echo "Listing current projects:"
./draheim-client -action=list
print_success "Client successfully connected to server"
echo ""

# Step 4: Deploy sample applications using local paths
print_step "4" "Deploying sample applications from local demo directory"

# Build demo apps first
print_info "Building demo applications..."
cd demo/sample-apps/hello-world && go build -o hello-world . && cd ../../..
cd demo/sample-apps/api-server && go build -o api-server . && cd ../../..
cd demo/sample-apps/counter-app && go build -o counter-app . && cd ../../..

# Deploy each app
print_info "Deploying Hello World app on port 8081..."
./draheim-client -action=deploy -name=hello-world -path="$(pwd)/demo/sample-apps/hello-world/hello-world" -port=8081
sleep 2

print_info "Deploying API Server app on port 8082..."
./draheim-client -action=deploy -name=api-server -path="$(pwd)/demo/sample-apps/api-server/api-server" -port=8082
sleep 2

print_info "Deploying Counter app on port 8083..."
./draheim-client -action=deploy -name=counter-app -path="$(pwd)/demo/sample-apps/counter-app/counter-app" -port=8083
sleep 2

print_success "All demo applications deployed successfully"
echo ""

# Step 5: Show running projects
print_step "5" "Checking deployed applications"
echo "Current running projects:"
./draheim-client -action=list
echo ""

# Step 6: Test the applications
print_step "6" "Testing deployed applications"

print_info "Testing Hello World app (http://localhost:8081)..."
if curl -s "http://localhost:8081/health" | grep -q "healthy"; then
    print_success "Hello World app is responding correctly"
else
    print_warning "Hello World app may not be ready yet"
fi

print_info "Testing API Server app (http://localhost:8082)..."
if curl -s "http://localhost:8082/health" | grep -q "healthy"; then
    print_success "API Server app is responding correctly"
else
    print_warning "API Server app may not be ready yet"
fi

print_info "Testing Counter app (http://localhost:8083)..."
if curl -s "http://localhost:8083/health" | grep -q "healthy"; then
    print_success "Counter app is responding correctly"
else
    print_warning "Counter app may not be ready yet"
fi
echo ""

# Step 7: Show logs
print_step "7" "Demonstrating log functionality"
print_info "Getting logs for hello-world app:"
./draheim-client -action=logs -name=hello-world | head -10
echo ""

# Step 8: Demonstrate web interface
print_step "8" "Web Interface Demo"
echo ""
echo -e "${PURPLE}🌐 Web Interface Information:${NC}"
echo -e "   ${CYAN}Dashboard:${NC}     http://localhost:8080"
echo -e "   ${CYAN}Configuration:${NC} http://localhost:8080/config"
echo ""
echo -e "${PURPLE}🎯 Demo Applications:${NC}"
echo -e "   ${CYAN}Hello World:${NC}   http://localhost:8081"
echo -e "   ${CYAN}API Server:${NC}    http://localhost:8082"
echo -e "   ${CYAN}Counter App:${NC}   http://localhost:8083"
echo ""

# Step 9: Interactive demo menu
print_step "9" "Interactive Demo Menu"
echo ""
echo "Choose an action:"
echo "1) View application logs"
echo "2) Stop an application"
echo "3) Restart an application"
echo "4) Show application status"
echo "5) Open web dashboard in browser (if available)"
echo "6) Run stress test on applications"
echo "7) Cleanup and exit"
echo ""

while true; do
    echo -n "Enter your choice (1-7): "
    read choice
    echo ""
    
    case $choice in
        1)
            echo "Available applications: hello-world, api-server, counter-app"
            echo -n "Enter app name: "
            read app_name
            echo ""
            print_info "Showing logs for $app_name:"
            ./draheim-client -action=logs -name=$app_name
            echo ""
            ;;
        2)
            echo "Available applications: hello-world, api-server, counter-app"
            echo -n "Enter app name to stop: "
            read app_name
            echo ""
            print_info "Stopping $app_name..."
            ./draheim-client -action=stop -name=$app_name
            print_success "$app_name stopped"
            echo ""
            ;;
        3)
            echo "Available applications: hello-world, api-server, counter-app"
            echo -n "Enter app name to restart: "
            read app_name
            echo ""
            print_info "Restarting $app_name..."
            case $app_name in
                hello-world)
                    ./draheim-client -action=deploy -name=hello-world -path="$(pwd)/demo/sample-apps/hello-world/hello-world" -port=8081
                    ;;
                api-server)
                    ./draheim-client -action=deploy -name=api-server -path="$(pwd)/demo/sample-apps/api-server/api-server" -port=8082
                    ;;
                counter-app)
                    ./draheim-client -action=deploy -name=counter-app -path="$(pwd)/demo/sample-apps/counter-app/counter-app" -port=8083
                    ;;
                *)
                    print_error "Unknown application: $app_name"
                    continue
                    ;;
            esac
            print_success "$app_name restarted"
            echo ""
            ;;
        4)
            print_info "Current application status:"
            ./draheim-client -action=list
            echo ""
            ;;
        5)
            if command -v xdg-open > /dev/null; then
                print_info "Opening web dashboard..."
                xdg-open http://localhost:8080
            elif command -v open > /dev/null; then
                print_info "Opening web dashboard..."
                open http://localhost:8080
            else
                print_warning "Cannot automatically open browser. Please visit: http://localhost:8080"
            fi
            echo ""
            ;;
        6)
            print_info "Running stress test on applications..."
            echo "Testing Hello World app:"
            for i in {1..5}; do
                response=$(curl -s "http://localhost:8081/health" || echo "failed")
                echo "  Request $i: $response"
            done
            echo ""
            echo "Testing API Server app:"
            for i in {1..5}; do
                response=$(curl -s "http://localhost:8082/tasks" || echo "failed")
                echo "  Request $i: $(echo $response | jq -r '.message' 2>/dev/null || echo $response)"
            done
            echo ""
            echo "Testing Counter app:"
            for i in {1..3}; do
                curl -s -X POST "http://localhost:8083/increment" > /dev/null
                response=$(curl -s "http://localhost:8083/api/count" || echo "failed")
                echo "  Increment $i: $(echo $response | jq -r '.count' 2>/dev/null || echo $response)"
            done
            print_success "Stress test completed"
            echo ""
            ;;
        7)
            break
            ;;
        *)
            print_error "Invalid choice. Please enter 1-7."
            ;;
    esac
done

# Cleanup
print_step "10" "Cleaning up demo environment"
print_info "Stopping demo applications..."
./draheim-client -action=stop -name=hello-world 2>/dev/null || true
./draheim-client -action=stop -name=api-server 2>/dev/null || true
./draheim-client -action=stop -name=counter-app 2>/dev/null || true

print_info "Stopping Draheim server..."
kill $SERVER_PID 2>/dev/null || true
sleep 2

print_info "Cleaning up tmux sessions..."
tmux kill-session -t hello-world 2>/dev/null || true
tmux kill-session -t api-server 2>/dev/null || true
tmux kill-session -t counter-app 2>/dev/null || true
tmux kill-session -t draheim 2>/dev/null || true

print_success "Demo cleanup completed"
echo ""

echo "🎬 =================================================="
echo "🎬  DEMO COMPLETED SUCCESSFULLY!"
echo "🎬 =================================================="
echo ""
echo -e "${GREEN}Thank you for trying the Draheim deployment system!${NC}"
echo ""
echo -e "${CYAN}Key features demonstrated:${NC}"
echo "  ✅ Server/client architecture"
echo "  ✅ Multiple application deployment"
echo "  ✅ Process management with tmux"
echo "  ✅ Log viewing and monitoring"
echo "  ✅ Web dashboard interface"
echo "  ✅ Health checks and status monitoring"
echo ""
echo -e "${YELLOW}To get started with your own applications:${NC}"
echo "  1. Run 'make build' to build the system"
echo "  2. Run 'make run' to start the server"
echo "  3. Visit http://localhost:8080 for the web interface"
echo "  4. Use './draheim-client' for command-line operations"
echo ""
echo -e "${BLUE}For more information, see the README.md file${NC}"
echo ""