package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config represents the server configuration
type Config struct {
	ServerPort    string   `json:"server_port"`
	RepoBasePath  string   `json:"repo_base_path"`
	AllowedHosts  []string `json:"allowed_hosts"`
	SSHKeyPath    string   `json:"ssh_key_path"`
	DefaultBranch string   `json:"default_branch"`
}

// LoadConfig loads configuration from file or creates default
func LoadConfig() *Config {
	configPath := "draheim-config.json"
	
	// Try to load existing config
	if data, err := ioutil.ReadFile(configPath); err == nil {
		var config Config
		if err := json.Unmarshal(data, &config); err == nil {
			return &config
		}
	}
	
	// Create default config
	config := &Config{
		ServerPort:    ":8080",
		RepoBasePath:  "/tmp/draheim-repos",
		AllowedHosts:  []string{"github.com", "gitlab.com", "bitbucket.org"},
		SSHKeyPath:    filepath.Join(os.Getenv("HOME"), ".ssh/id_rsa"),
		DefaultBranch: "main",
	}
	
	// Save default config
	if data, err := json.MarshalIndent(config, "", "  "); err == nil {
		ioutil.WriteFile(configPath, data, 0644)
	}
	
	return config
}

// Project represents a deployed Go application
type Project struct {
	Name        string
	Status      string
	Port        int
	PID         int
	LastUpdated time.Time
	LogFile     string
	BinaryPath  string
	GitRepo     string
	GitBranch   string
	WorkingDir  string
}

// ProjectManager handles deployment and management of Go applications
type ProjectManager struct {
	projects map[string]*Project
	config   *Config
}

func NewProjectManager(config *Config) *ProjectManager {
	return &ProjectManager{
		projects: make(map[string]*Project),
		config:   config,
	}
}

func (pm *ProjectManager) GetProjects() map[string]*Project {
	return pm.projects
}

func (pm *ProjectManager) StartProject(name, binaryPath string, port int) error {
	return pm.StartProjectWithGit(name, binaryPath, "", "", port)
}

func (pm *ProjectManager) StartProjectWithGit(name, binaryPath, gitRepo, gitBranch string, port int) error {
	// Check if tmux session exists
	cmd := exec.Command("tmux", "has-session", "-t", name)
	if err := cmd.Run(); err == nil {
		return fmt.Errorf("project %s is already running", name)
	}

	workingDir := binaryPath
	
	// If Git repository is specified, handle cloning/updating
	if gitRepo != "" {
		if gitBranch == "" {
			gitBranch = pm.config.DefaultBranch
		}
		
		// Create working directory path using config
		workingDir = filepath.Join(pm.config.RepoBasePath, name)
		
		// Clone or update repository
		if err := pm.CloneOrUpdateRepo(gitRepo, workingDir, gitBranch); err != nil {
			return err
		}
		
		// Build the project
		binaryName := name
		if err := pm.BuildProject(workingDir, binaryName); err != nil {
			return err
		}
		
		binaryPath = filepath.Join(workingDir, binaryName)
	}

	// Create new tmux session and start the application
	var tmuxCmd string
	if gitRepo != "" {
		tmuxCmd = fmt.Sprintf("cd %s && ./%s", workingDir, name)
	} else {
		tmuxCmd = fmt.Sprintf("cd %s && ./%s", filepath.Dir(binaryPath), filepath.Base(binaryPath))
	}
	
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
		GitRepo:     gitRepo,
		GitBranch:   gitBranch,
		WorkingDir:  workingDir,
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

// CloneOrUpdateRepo clones a repository or pulls updates if it already exists
func (pm *ProjectManager) CloneOrUpdateRepo(repoURL, workingDir, branch string) error {
	// Ensure base directory exists
	if err := os.MkdirAll(pm.config.RepoBasePath, 0755); err != nil {
		return fmt.Errorf("failed to create repo base path: %v", err)
	}
	
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		// Repository doesn't exist, clone it
		log.Printf("Cloning repository %s to %s", repoURL, workingDir)
		
		// Set up Git command with SSH key if available
		cmd := exec.Command("git", "clone", "-b", branch, repoURL, workingDir)
		if pm.config.SSHKeyPath != "" {
			if _, err := os.Stat(pm.config.SSHKeyPath); err == nil {
				cmd.Env = append(os.Environ(), 
					fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", pm.config.SSHKeyPath))
			}
		}
		
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %v", err)
		}
	} else {
		// Repository exists, pull updates
		log.Printf("Updating repository in %s", workingDir)
		cmd := exec.Command("git", "-C", workingDir, "pull", "origin", branch)
		
		// Set up SSH key for pull as well
		if pm.config.SSHKeyPath != "" {
			if _, err := os.Stat(pm.config.SSHKeyPath); err == nil {
				cmd.Env = append(os.Environ(), 
					fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no", pm.config.SSHKeyPath))
			}
		}
		
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to pull updates: %v", err)
		}
	}
	return nil
}

// BuildProject builds a Go project in the working directory
func (pm *ProjectManager) BuildProject(workingDir, binaryName string) error {
	log.Printf("Building project in %s", workingDir)
	cmd := exec.Command("go", "build", "-o", binaryName, ".")
	cmd.Dir = workingDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build project: %v", err)
	}
	return nil
}

var pm *ProjectManager
var config *Config

func main() {
	config = LoadConfig()
	pm = NewProjectManager(config)

	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	// Routes
	http.HandleFunc("/", dashboardHandler)
	http.HandleFunc("/projects", projectsHandler)
	http.HandleFunc("/deploy", deployHandler)
	http.HandleFunc("/stop", stopHandler)
	http.HandleFunc("/logs", logsHandler)
	http.HandleFunc("/update", updateHandler)
	http.HandleFunc("/config", configHandler)

	log.Printf("Dra heim dashboard starting on http://localhost%s", config.ServerPort)
	log.Printf("Repository base path: %s", config.RepoBasePath)
	log.Printf("SSH key path: %s", config.SSHKeyPath)
	log.Fatal(http.ListenAndServe(config.ServerPort, nil))
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Dra heim - Application Dashboard</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
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
            <p><a href="/config" class="btn btn-primary">⚙️ Configuration</a></p>
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
                        <label>Binary Path (or leave empty for Git deployment):</label>
                        <input type="text" name="path" placeholder="/path/to/binary" list="path-suggestions" autocomplete="off">
                        <datalist id="path-suggestions">
                            <option value="/home/bitraf/apps/">
                            <option value="/opt/">
                            <option value="/usr/local/bin/">
                            <option value="./bin/">
                        </datalist>
                    </div>
                    <div class="form-group">
                        <label>Git Repository URL (optional):</label>
                        <input type="url" name="git_repo" placeholder="https://github.com/user/repo.git">
                    </div>
                    <div class="form-group">
                        <label>Git Branch (default: main):</label>
                        <input type="text" name="git_branch" placeholder="main" value="main">
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
            <div id="projects-list" hx-get="/projects" hx-trigger="every 5s">
                {{template "projects" .}}
            </div>
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
            <th>Git Repo</th>
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
            <td>{{if .GitRepo}}{{.GitBranch}}@{{.GitRepo}}{{else}}Binary{{end}}</td>
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
                {{if .GitRepo}}
                    <form action="/update" method="post" style="display: inline;">
                        <input type="hidden" name="name" value="{{.Name}}">
                        <button type="submit" class="btn btn-success">Update</button>
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
            <th>Git Repo</th>
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
            <td>{{if .GitRepo}}{{.GitBranch}}@{{.GitRepo}}{{else}}Binary{{end}}</td>
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
                {{if .GitRepo}}
                    <button class="btn btn-success" 
                            hx-post="/update" 
                            hx-vals='{"name": "{{.Name}}"}'
                            hx-target="#projects-list">Update</button>
                {{end}}
                <button class="btn btn-primary" 
                        hx-get="/logs?project={{.Name}}" 
                        hx-target="#logs-{{.Name}}" 
                        hx-swap="innerHTML">Logs</button>
            </td>
        </tr>
        <tr id="logs-{{.Name}}" style="display: none;">
            <td colspan="6"></td>
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
	gitRepo := r.FormValue("git_repo")
	gitBranch := r.FormValue("git_branch")
	portStr := r.FormValue("port")
	restart := r.FormValue("restart") == "true"

	if name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	// For restart, we don't need path or git info
	if !restart && path == "" && gitRepo == "" {
		http.Error(w, "Either binary path or Git repository is required for new deployments", http.StatusBadRequest)
		return
	}

	port := 8081 // Default port
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// If restart, use existing project info
	if restart {
		if existingProject, exists := pm.projects[name]; exists {
			path = existingProject.BinaryPath
			port = existingProject.Port
			gitRepo = existingProject.GitRepo
			gitBranch = existingProject.GitBranch
		} else {
			http.Error(w, "Project not found for restart", http.StatusBadRequest)
			return
		}
	}

	// Set default branch if not specified
	if gitBranch == "" {
		gitBranch = config.DefaultBranch
	}

	var err error
	if gitRepo != "" {
		err = pm.StartProjectWithGit(name, path, gitRepo, gitBranch, port)
	} else {
		err = pm.StartProject(name, path, port)
	}

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

func updateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	project, exists := pm.projects[name]
	if !exists {
		http.Error(w, "Project not found", http.StatusBadRequest)
		return
	}

	// Only update if it's a Git-based project
	if project.GitRepo == "" {
		http.Error(w, "Project is not Git-based", http.StatusBadRequest)
		return
	}

	// Stop the project first
	if err := pm.StopProject(name); err != nil {
		log.Printf("Error stopping project %s for update: %v", name, err)
	}

	// Update the repository
	if err := pm.CloneOrUpdateRepo(project.GitRepo, project.WorkingDir, project.GitBranch); err != nil {
		log.Printf("Error updating repository for %s: %v", name, err)
		http.Error(w, fmt.Sprintf("Failed to update repository: %v", err), http.StatusInternalServerError)
		return
	}

	// Rebuild the project
	if err := pm.BuildProject(project.WorkingDir, name); err != nil {
		log.Printf("Error building project %s: %v", name, err)
		http.Error(w, fmt.Sprintf("Failed to build project: %v", err), http.StatusInternalServerError)
		return
	}

	// Restart the project
	if err := pm.StartProjectWithGit(name, project.BinaryPath, project.GitRepo, project.GitBranch, project.Port); err != nil {
		log.Printf("Error restarting project %s after update: %v", name, err)
		http.Error(w, fmt.Sprintf("Failed to restart project: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully updated project: %s", name)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Configuration - Dra heim</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; }
        .header { background: #2c3e50; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .card { background: white; padding: 20px; margin: 10px 0; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .btn { padding: 8px 16px; margin: 5px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; }
        .btn-primary { background: #3498db; color: white; }
        .form-group { margin: 10px 0; }
        input, textarea { padding: 8px; margin: 5px; border: 1px solid #ddd; border-radius: 4px; width: 100%; }
        pre { background: #2c3e50; color: #ecf0f1; padding: 15px; border-radius: 4px; overflow-x: auto; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>⚙️ Configuration</h1>
            <p>Server Configuration for Dra heim</p>
            <a href="/" class="btn btn-primary">← Back to Dashboard</a>
        </div>
        
        <div class="card">
            <h2>Current Configuration</h2>
            <pre>{{.ConfigJSON}}</pre>
        </div>
        
        <div class="card">
            <h2>Configuration Details</h2>
            <p><strong>Server Port:</strong> {{.Config.ServerPort}}</p>
            <p><strong>Repository Base Path:</strong> {{.Config.RepoBasePath}}</p>
            <p><strong>SSH Key Path:</strong> {{.Config.SSHKeyPath}}</p>
            <p><strong>Default Branch:</strong> {{.Config.DefaultBranch}}</p>
            <p><strong>Allowed Hosts:</strong> {{range .Config.AllowedHosts}}{{.}} {{end}}</p>
        </div>
    </div>
</body>
</html>
`
		
		configJSON, _ := json.MarshalIndent(config, "", "  ")
		
		data := struct {
			Config     *Config
			ConfigJSON string
		}{
			Config:     config,
			ConfigJSON: string(configJSON),
		}
		
		t, _ := template.New("config").Parse(tmpl)
		t.Execute(w, data)
	}
}