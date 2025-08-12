package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Project represents a deployed Go application
type Project struct {
	Name        string
	Status      string
	Port        int
	PID         int
	LastUpdated time.Time
	LogFile     string
	BinaryPath  string
}

// ProjectManager handles deployment and management of Go applications
type ProjectManager struct {
	projects map[string]*Project
}

func NewProjectManager() *ProjectManager {
	return &ProjectManager{
		projects: make(map[string]*Project),
	}
}

func (pm *ProjectManager) GetProjects() map[string]*Project {
	return pm.projects
}

func (pm *ProjectManager) StartProject(name, binaryPath string, port int) error {
	// Check if tmux session exists
	cmd := exec.Command("tmux", "has-session", "-t", name)
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("project %s is already running", name)
	}

	// Create new tmux session and start the application
	tmuxCmd := fmt.Sprintf("cd %s && ./%s", binaryPath, name)
	cmd = exec.Command("tmux", "new-session", "-d", "-s", name, tmuxCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start project %s: %v", name, err)
	}

	// Add to projects map
	pm.projects[name] = &Project{
		Name:        name,
		Status:      "running",
		Port:        port,
		LastUpdated: time.Now(),
		LogFile:     fmt.Sprintf("/tmp/%s.log", name),
		BinaryPath:  binaryPath,
	}

	return nil
}

func (pm *ProjectManager) StopProject(name string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop project %s: %v", name, err)
	}

	if project, exists := pm.projects[name]; exists {
		project.Status = "stopped"
		project.LastUpdated = time.Now()
	}

	return nil
}

func (pm *ProjectManager) GetProjectStatus(name string) (string, error) {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	if err := cmd.Run(); err != nil {
		return "stopped", nil
	}
	return "running", nil
}

var pm *ProjectManager

func main() {
	pm = NewProjectManager()

	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	// Routes
	http.HandleFunc("/", dashboardHandler)
	http.HandleFunc("/projects", projectsHandler)
	http.HandleFunc("/deploy", deployHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/logs", logsHandler)

	port := ":8080"
	log.Printf("Dra heim dashboard starting on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Dra heim - Application Dashboard</title>
    <meta http-equiv="refresh" content="10">
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: #2c3e50; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .card { background: white; padding: 20px; margin: 10px 0; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .status-running { color: #27ae60; font-weight: bold; }
        .status-stopped { color: #e74c3c; font-weight: bold; }
        .btn { padding: 8px 16px; margin: 5px; border: none; border-radius: 4px; cursor: pointer; }
        .btn-primary { background: #3498db; color: white; }
        .btn-danger { background: #e74c3c; color: white; }
        .btn-success { background: #27ae60; color: white; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #f8f9fa; }
        .deploy-form { background: #ecf0f1; padding: 15px; border-radius: 4px; margin: 10px 0; }
        .form-group { margin: 10px 0; }
        input, select { padding: 8px; margin: 5px; border: 1px solid #ddd; border-radius: 4px; }
        .logs { background: #2c3e50; color: #ecf0f1; padding: 10px; border-radius: 4px; font-family: monospace; max-height: 300px; overflow-y: scroll; margin: 10px 0; }
        .refresh-info { color: #7f8c8d; font-size: 0.9em; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 Dra heim</h1>
            <p>Go Application Deployment Dashboard for Bitraf's Heim Server</p>
            <p class="refresh-info">⚡ Auto-refreshes every 10 seconds</p>
        </div>

        <div class="card">
            <h2>Deploy New Application</h2>
            <div class="deploy-form">
                <form action="/deploy" method="post">
                    <div class="form-group">
                        <label>Project Name:</label>
                        <input type="text" name="name" required placeholder="my-app">
                    </div>
                    <div class="form-group">
                        <label>Binary Path:</label>
                        <input type="text" name="path" required placeholder="/path/to/binary">
                    </div>
                    <div class="form-group">
                        <label>Port:</label>
                        <input type="number" name="port" required placeholder="8081">
                    </div>
                    <button type="submit" class="btn btn-success">Deploy Application</button>
                </form>
            </div>
        </div>

        <div class="card">
            <h2>Active Projects</h2>
            {{template "projects" .}}
        </div>
    </div>
</body>
</html>

{{define "projects"}}
<table>
    <thead>
        <tr>
            <th>Project Name</th>
            <th>Status</th>
            <th>Port</th>
            <th>Last Updated</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        {{range .}}
        <tr>
            <td>{{.Name}}</td>
            <td class="status-{{.Status}}">{{.Status}}</td>
            <td>{{.Port}}</td>
            <td>{{.LastUpdated.Format "15:04:05"}}</td>
            <td>
                {{if eq .Status "running"}}
                    <form action="/stop" method="post" style="display: inline;">
                        <input type="hidden" name="name" value="{{.Name}}">
                        <button type="submit" class="btn btn-danger">Stop</button>
                    </form>
                {{else}}
                    <form action="/deploy" method="post" style="display: inline;">
                        <input type="hidden" name="name" value="{{.Name}}">
                        <input type="hidden" name="restart" value="true">
                        <button type="submit" class="btn btn-primary">Start</button>
                    </form>
                {{end}}
                <a href="/logs?project={{.Name}}" class="btn btn-primary">Logs</a>
            </td>
        </tr>
        {{end}}
    </tbody>
</table>
{{if eq (len .) 0}}
<p>No projects deployed yet. Use the form above to deploy your first Go application.</p>
{{end}}
{{end}}
`

	// Update project statuses
	projects := make([]*Project, 0, len(pm.GetProjects()))
	for name, project := range pm.GetProjects() {
		status, _ := pm.GetProjectStatus(name)
		project.Status = status
		project.LastUpdated = time.Now()
		projects = append(projects, project)
	}

	t, _ := template.New("dashboard").Parse(tmpl)
	t.Execute(w, projects)
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	// Update project statuses
	for name, project := range pm.GetProjects() {
		status, _ := pm.GetProjectStatus(name)
		project.Status = status
		project.LastUpdated = time.Now()
	}

	tmpl := `
<table>
    <thead>
        <tr>
            <th>Project Name</th>
            <th>Status</th>
            <th>Port</th>
            <th>Last Updated</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        {{range .}}
        <tr>
            <td>{{.Name}}</td>
            <td class="status-{{.Status}}">{{.Status}}</td>
            <td>{{.Port}}</td>
            <td>{{.LastUpdated.Format "15:04:05"}}</td>
            <td>
                {{if eq .Status "running"}}
                    <button class="btn btn-danger" 
                            hx-post="/stop" 
                            hx-vals='{"name": "{{.Name}}"}'
                            hx-target="#projects-list">Stop</button>
                {{else}}
                    <button class="btn btn-primary" 
                            hx-post="/deploy" 
                            hx-vals='{"name": "{{.Name}}", "restart": "true"}'
                            hx-target="#projects-list">Start</button>
                {{end}}
                <button class="btn btn-primary" 
                        hx-get="/logs?project={{.Name}}" 
                        hx-target="#logs-{{.Name}}" 
                        hx-swap="innerHTML">Logs</button>
            </td>
        </tr>
        <tr id="logs-{{.Name}}" style="display: none;">
            <td colspan="5"></td>
        </tr>
        {{end}}
    </tbody>
</table>
{{if eq (len .) 0}}
<p>No projects deployed yet. Use the form above to deploy your first Go application.</p>
{{end}}
`

	projects := make([]*Project, 0, len(pm.GetProjects()))
	for _, project := range pm.GetProjects() {
		projects = append(projects, project)
	}

	t, _ := template.New("projects").Parse(tmpl)
	t.Execute(w, projects)
}

func deployHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	path := r.FormValue("path")
	portStr := r.FormValue("port")
	restart := r.FormValue("restart") == "true"

	if name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	// For restart, we don't need path
	if !restart && path == "" {
		http.Error(w, "Binary path is required for new deployments", http.StatusBadRequest)
		return
	}

	port := 8081 // Default port
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// If restart, use existing project path
	if restart {
		if existingProject, exists := pm.projects[name]; exists {
			path = existingProject.BinaryPath
			port = existingProject.Port
		} else {
			http.Error(w, "Project not found for restart", http.StatusBadRequest)
			return
		}
	}

	err := pm.StartProject(name, path, port)
	if err != nil {
		log.Printf("Error deploying project %s: %v", name, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully deployed project: %s", name)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	err := pm.StopProject(name)
	if err != nil {
		log.Printf("Error stopping project %s: %v", name, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully stopped project: %s", name)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	projectName := r.URL.Query().Get("project")
	if projectName == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	// Get tmux session output
	cmd := exec.Command("tmux", "capture-pane", "-t", projectName, "-p")
	output, err := cmd.Output()
	if err != nil {
		output = []byte(fmt.Sprintf("Error getting logs: %v", err))
	}

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Logs for {{.Project}} - Dra heim</title>
    <meta http-equiv="refresh" content="5">
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: #2c3e50; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .logs { background: #2c3e50; color: #ecf0f1; padding: 20px; border-radius: 4px; font-family: monospace; white-space: pre-wrap; word-wrap: break-word; }
        .btn { padding: 8px 16px; margin: 5px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; }
        .btn-primary { background: #3498db; color: white; }
        .refresh-info { color: #7f8c8d; font-size: 0.9em; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📋 Logs for {{.Project}}</h1>
            <p class="refresh-info">⚡ Auto-refreshes every 5 seconds</p>
            <a href="/" class="btn btn-primary">← Back to Dashboard</a>
        </div>
        <div class="logs">{{.Output}}</div>
    </div>
</body>
</html>
`

	data := struct {
		Project string
		Output  string
	}{
		Project: projectName,
		Output:  strings.TrimSpace(string(output)),
	}

	t, _ := template.New("logs").Parse(tmpl)
	t.Execute(w, data)
}