package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"crypto/subtle"
	"strings"
	"time"
)

// Config represents the server configuration
type Config struct {
	ServerPort     string   `json:"server_port"`
	RepoBasePath   string   `json:"repo_base_path"`
	AllowedHosts   []string `json:"allowed_hosts"`
	SSHKeyPath     string   `json:"ssh_key_path"`
	DefaultBranch  string   `json:"default_branch"`
	SecretsKeyPath string   `json:"secrets_key_path"`
	// APIKey gates every endpoint. Empty means the server refuses to
	// start: an unauthenticated deploy endpoint on a shared host is a
	// remote shell for anyone who can reach the port.
	APIKey string `json:"api_key"`
}

// Secret represents an encrypted secret
type Secret struct {
	Key         string `json:"key"`
	Value       string `json:"value"` // This will be encrypted
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// SecretsManager handles encrypted secret storage
type SecretsManager struct {
	secretsFile string
	keyFile     string
	secrets     map[string]*Secret
}

// NewSecretsManager creates a new secrets manager
func NewSecretsManager(keyPath string) *SecretsManager {
	return &SecretsManager{
		secretsFile: "draheim-secrets.json",
		keyFile:     keyPath,
		secrets:     make(map[string]*Secret),
	}
}

// generateKey generates a 32-byte key for AES-256
func (sm *SecretsManager) generateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// getOrCreateKey gets existing key or creates a new one
func (sm *SecretsManager) getOrCreateKey() ([]byte, error) {
	if _, err := os.Stat(sm.keyFile); os.IsNotExist(err) {
		// Generate new key
		key, err := sm.generateKey()
		if err != nil {
			return nil, err
		}
		
		// Save key to file (base64 encoded)
		keyStr := base64.StdEncoding.EncodeToString(key)
		if err := ioutil.WriteFile(sm.keyFile, []byte(keyStr), 0600); err != nil {
			return nil, err
		}
		
		log.Printf("Generated new encryption key at: %s", sm.keyFile)
		return key, nil
	}
	
	// Load existing key
	keyData, err := ioutil.ReadFile(sm.keyFile)
	if err != nil {
		return nil, err
	}
	
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyData)))
	if err != nil {
		return nil, err
	}
	
	return key, nil
}

// encrypt encrypts a string using AES-256-GCM
func (sm *SecretsManager) encrypt(plaintext string) (string, error) {
	key, err := sm.getOrCreateKey()
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a string using AES-256-GCM
func (sm *SecretsManager) decrypt(encryptedText string) (string, error) {
	key, err := sm.getOrCreateKey()
	if err != nil {
		return "", err
	}
	
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

// LoadSecrets loads secrets from encrypted file
func (sm *SecretsManager) LoadSecrets() error {
	if _, err := os.Stat(sm.secretsFile); os.IsNotExist(err) {
		// No secrets file exists yet
		return nil
	}
	
	data, err := ioutil.ReadFile(sm.secretsFile)
	if err != nil {
		return err
	}
	
	var encryptedSecrets map[string]*Secret
	if err := json.Unmarshal(data, &encryptedSecrets); err != nil {
		return err
	}
	
	// Decrypt secrets
	sm.secrets = make(map[string]*Secret)
	for key, secret := range encryptedSecrets {
		decryptedValue, err := sm.decrypt(secret.Value)
		if err != nil {
			log.Printf("Warning: Could not decrypt secret %s: %v", key, err)
			continue
		}
		
		sm.secrets[key] = &Secret{
			Key:         secret.Key,
			Value:       decryptedValue,
			Description: secret.Description,
			CreatedAt:   secret.CreatedAt,
			UpdatedAt:   secret.UpdatedAt,
		}
	}
	
	return nil
}

// SaveSecrets saves secrets to encrypted file
func (sm *SecretsManager) SaveSecrets() error {
	encryptedSecrets := make(map[string]*Secret)
	
	for key, secret := range sm.secrets {
		encryptedValue, err := sm.encrypt(secret.Value)
		if err != nil {
			return err
		}
		
		encryptedSecrets[key] = &Secret{
			Key:         secret.Key,
			Value:       encryptedValue,
			Description: secret.Description,
			CreatedAt:   secret.CreatedAt,
			UpdatedAt:   secret.UpdatedAt,
		}
	}
	
	data, err := json.MarshalIndent(encryptedSecrets, "", "  ")
	if err != nil {
		return err
	}
	
	return ioutil.WriteFile(sm.secretsFile, data, 0600)
}

// AddSecret adds a new secret
func (sm *SecretsManager) AddSecret(key, value, description string) error {
	now := time.Now().Format(time.RFC3339)
	
	sm.secrets[key] = &Secret{
		Key:         key,
		Value:       value,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	
	return sm.SaveSecrets()
}

// GetSecret retrieves a secret value
func (sm *SecretsManager) GetSecret(key string) (string, bool) {
	secret, exists := sm.secrets[key]
	if !exists {
		return "", false
	}
	return secret.Value, true
}

// ListSecrets returns all secrets (without values for security)
func (sm *SecretsManager) ListSecrets() map[string]*Secret {
	result := make(map[string]*Secret)
	for key, secret := range sm.secrets {
		result[key] = &Secret{
			Key:         secret.Key,
			Value:       "[HIDDEN]",
			Description: secret.Description,
			CreatedAt:   secret.CreatedAt,
			UpdatedAt:   secret.UpdatedAt,
		}
	}
	return result
}

// DeleteSecret removes a secret
func (sm *SecretsManager) DeleteSecret(key string) error {
	delete(sm.secrets, key)
	return sm.SaveSecrets()
}

// GetSecretsForProject returns environment variables for a project
func (sm *SecretsManager) GetSecretsForProject(projectName string) []string {
	var envVars []string
	
	// Add project-specific secrets (prefixed with PROJECT_NAME_)
	prefix := strings.ToUpper(projectName) + "_"
	for key, secret := range sm.secrets {
		if strings.HasPrefix(strings.ToUpper(key), prefix) {
			envKey := strings.TrimPrefix(strings.ToUpper(key), prefix)
			envVars = append(envVars, fmt.Sprintf("%s=%s", envKey, secret.Value))
		}
	}
	
	// Add global secrets (no prefix)
	for key, secret := range sm.secrets {
		if !strings.Contains(key, "_") {
			envVars = append(envVars, fmt.Sprintf("%s=%s", strings.ToUpper(key), secret.Value))
		}
	}
	
	return envVars
}

// finnSSHNøkkel picks a key that actually exists. The default was
// hardcoded to id_rsa, which many machines no longer have - ed25519 has
// been the default for new keys for years - so private clones failed with
// a confusing git error rather than a missing-file one.
func finnSSHNøkkel() string {
	heim := os.Getenv("HOME")
	for _, n := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		bane := filepath.Join(heim, ".ssh", n)
		if _, err := os.Stat(bane); err == nil {
			return bane
		}
	}
	return filepath.Join(heim, ".ssh", "id_ed25519")
}

func nyNøkkel() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// reProjectName is deliberately strict. The project name reaches tmux,
// the filesystem and (previously) a shell string, so anything outside
// this set is rejected rather than escaped.
var reProjectName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

func validProjectName(name string) error {
	if !reProjectName.MatchString(name) {
		return fmt.Errorf("invalid project name %q: use letters, digits, dot, dash, underscore (max 64)", name)
	}
	return nil
}

// hostAllowed enforces AllowedHosts, which until now was declared,
// displayed on the config page, and never actually checked.
func (pm *ProjectManager) hostAllowed(repoURL string) error {
	if len(pm.config.AllowedHosts) == 0 {
		return fmt.Errorf("no allowed_hosts configured; refusing to clone %q", repoURL)
	}
	host := ""
	if u, err := url.Parse(repoURL); err == nil && u.Host != "" {
		host = u.Hostname()
	} else if i := strings.Index(repoURL, "@"); i >= 0 {
		// scp-form: git@github.com:user/repo.git
		rest := repoURL[i+1:]
		host = strings.SplitN(rest, ":", 2)[0]
	}
	if host == "" {
		return fmt.Errorf("could not determine host from %q", repoURL)
	}
	for _, a := range pm.config.AllowedHosts {
		if strings.EqualFold(host, a) {
			return nil
		}
	}
	return fmt.Errorf("host %q is not in allowed_hosts", host)
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
	
	// Create default config.
	//
	// Bound to loopback, not ":8080". The dashboard can deploy and run
	// arbitrary code, so it has no business listening on every interface of
	// a shared host by default; reach it over an SSH tunnel, or set the
	// address deliberately.
	//
	// A fresh key is generated rather than left blank, so a new install is
	// never briefly open while someone gets around to filling it in.
	config := &Config{
		ServerPort:     "127.0.0.1:8080",
		RepoBasePath:   "/tmp/draheim-repos",
		AllowedHosts:   []string{"github.com", "gitlab.com", "bitbucket.org"},
		SSHKeyPath:     finnSSHNøkkel(),
		DefaultBranch:  "main",
		SecretsKeyPath: "draheim-secrets.key",
		APIKey:         nyNøkkel(),
	}
	
	// Save default config. 0600: it holds the API key.
	if data, err := json.MarshalIndent(config, "", "  "); err == nil {
		ioutil.WriteFile(configPath, data, 0600)
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
	BuildPath   string
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
	return pm.StartProjectWithGit(name, binaryPath, "", "", "", port)
}

func (pm *ProjectManager) StartProjectWithGit(name, binaryPath, gitRepo, gitBranch, buildPath string, port int) error {
	if err := validProjectName(name); err != nil {
		return err
	}
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
		if err := pm.BuildProject(workingDir, binaryName, buildPath); err != nil {
			return err
		}
		
		binaryPath = filepath.Join(workingDir, binaryName)
	}

	// Start the application in its own tmux session.
	//
	// The command is passed as argv, not as a shell string. It used to be
	// built with fmt.Sprintf and handed to tmux to interpret, so a project
	// name containing ';' ran whatever followed it - and every secret value
	// was spliced in unquoted, which both broke on spaces and injected.
	//
	// Secrets go through the environment (tmux -e) instead of the command
	// line, so they no longer show up in `ps` for every user on the host.
	runDir := workingDir
	binary := name
	if gitRepo == "" {
		runDir = filepath.Dir(binaryPath)
		binary = filepath.Base(binaryPath)
	}

	args := []string{"new-session", "-d", "-s", name, "-c", runDir}
	for _, kv := range secretsManager.GetSecretsForProject(name) {
		args = append(args, "-e", kv)
	}
	args = append(args, "./"+binary)

	cmd = exec.Command("tmux", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start project %s: %v: %s", name, err, strings.TrimSpace(string(out)))
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
		BuildPath:   buildPath,
	}

	return nil
}

func (pm *ProjectManager) StopProject(name string) error {
	if err := validProjectName(name); err != nil {
		return err
	}
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
	if err := pm.hostAllowed(repoURL); err != nil {
		return err
	}
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
// BuildProject builds the package at buildPath relative to the repo root.
// It used to hardcode ".", which fails for every repo whose main package
// lives in a subdirectory - the common Go layout, e.g. ./cmd/server.
func (pm *ProjectManager) BuildProject(workingDir, binaryName, buildPath string) error {
	if buildPath == "" {
		buildPath = "."
	}
	if filepath.IsAbs(buildPath) || strings.Contains(buildPath, "..") {
		return fmt.Errorf("invalid build path %q: must be relative and inside the repo", buildPath)
	}
	log.Printf("Building %s in %s", buildPath, workingDir)
	cmd := exec.Command("go", "build", "-o", binaryName, "./"+filepath.ToSlash(filepath.Clean(buildPath)))
	cmd.Dir = workingDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build project: %v", err)
	}
	return nil
}

var pm *ProjectManager
var config *Config
var secretsManager *SecretsManager

// krev wraps a handler so it only runs for a caller that presents the
// API key, either as "Authorization: Bearer <key>" or the draheim_key
// cookie (so the dashboard works in a browser after one login).
//
// subtle.ConstantTimeCompare, not ==: string comparison returns early on
// the first differing byte, which leaks the key one byte at a time to
// anyone willing to time the responses.
func krev(neste http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gitt := ""
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			gitt = strings.TrimPrefix(h, "Bearer ")
		} else if c, err := r.Cookie("draheim_key"); err == nil {
			gitt = c.Value
		} else if q := r.URL.Query().Get("key"); q != "" {
			gitt = q
		}
		if subtle.ConstantTimeCompare([]byte(gitt), []byte(config.APIKey)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="draheim"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("key") != "" {
			http.SetCookie(w, &http.Cookie{Name: "draheim_key", Value: gitt,
				Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		}
		neste(w, r)
	}
}

func main() {
	config = LoadConfig()
	if config.APIKey == "" {
		log.Fatal("api_key is empty in draheim-config.json - refusing to start. " +
			"Generate one with:  openssl rand -hex 32")
	}
	pm = NewProjectManager(config)
	
	// Initialize secrets manager
	secretsManager = NewSecretsManager(config.SecretsKeyPath)
	if err := secretsManager.LoadSecrets(); err != nil {
		log.Printf("Warning: Could not load secrets: %v", err)
	} else {
		log.Printf("Secrets manager initialized")
	}

	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	// Routes. Every one of them is behind krev(): the client config has
	// carried an api_key since the beginning, but nothing ever sent it and
	// nothing ever checked it, so /deploy would clone and run any repo for
	// anyone who could reach the port.
	http.HandleFunc("/", krev(dashboardHandler))
	http.HandleFunc("/projects", krev(projectsHandler))
	http.HandleFunc("/deploy", krev(deployHandler))
	http.HandleFunc("/stop", krev(stopHandler))
	http.HandleFunc("/logs", krev(logsHandler))
	http.HandleFunc("/update", krev(updateHandler))
	http.HandleFunc("/config", krev(configHandler))
	http.HandleFunc("/secrets", krev(secretsHandler))
	http.HandleFunc("/secrets/add", krev(addSecretHandler))
	http.HandleFunc("/secrets/delete", krev(deleteSecretHandler))

	log.Printf("Dra heim dashboard starting on http://localhost%s", config.ServerPort)
	log.Printf("Repository base path: %s", config.RepoBasePath)
	log.Printf("SSH key path: %s", config.SSHKeyPath)
	log.Printf("Secrets key path: %s", config.SecretsKeyPath)
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
                        <label>Build path (default: repo root):</label>
                        <input type="text" name="build_path" placeholder="cmd/server">
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
	buildPath := r.FormValue("build_path")
	portStr := r.FormValue("port")
	restart := r.FormValue("restart") == "true"

	if err := validProjectName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
			buildPath = existingProject.BuildPath
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
		err = pm.StartProjectWithGit(name, path, gitRepo, gitBranch, buildPath, port)
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
	if err := pm.BuildProject(project.WorkingDir, name, project.BuildPath); err != nil {
		log.Printf("Error building project %s: %v", name, err)
		http.Error(w, fmt.Sprintf("Failed to build project: %v", err), http.StatusInternalServerError)
		return
	}

	// Restart the project
	if err := pm.StartProjectWithGit(name, project.BinaryPath, project.GitRepo, project.GitBranch, project.BuildPath, project.Port); err != nil {
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
            <a href="/secrets" class="btn btn-primary">🔐 Manage Secrets</a>
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
            <p><strong>Secrets Key Path:</strong> {{.Config.SecretsKeyPath}}</p>
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

func secretsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Secrets Management - Dra heim</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1000px; margin: 0 auto; }
        .header { background: #2c3e50; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .card { background: white; padding: 20px; margin: 10px 0; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .btn { padding: 8px 16px; margin: 5px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; }
        .btn-primary { background: #3498db; color: white; }
        .btn-success { background: #27ae60; color: white; }
        .btn-danger { background: #e74c3c; color: white; }
        .form-group { margin: 15px 0; }
        input, textarea { padding: 8px; margin: 5px; border: 1px solid #ddd; border-radius: 4px; width: 100%; box-sizing: border-box; }
        table { width: 100%; border-collapse: collapse; margin: 10px 0; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #f8f9fa; }
        .secret-form { background: #ecf0f1; padding: 20px; border-radius: 8px; margin: 10px 0; }
        .warning { background: #f39c12; color: white; padding: 15px; border-radius: 4px; margin: 10px 0; }
        .info { background: #3498db; color: white; padding: 15px; border-radius: 4px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔐 Secrets Management</h1>
            <p>Secure management of environment variables and sensitive data</p>
            <a href="/" class="btn btn-primary">← Back to Dashboard</a>
            <a href="/config" class="btn btn-primary">⚙️ Configuration</a>
        </div>

        <div class="info">
            <strong>ℹ️ How secrets work:</strong><br>
            • Global secrets are available to all projects (no prefix)<br>
            • Project-specific secrets use format: PROJECT_NAME_SECRET_KEY<br>
            • All secrets are encrypted at rest using AES-256-GCM<br>
            • Secrets are injected as environment variables when starting projects
        </div>

        <div class="card">
            <h2>Add New Secret</h2>
            <div class="secret-form">
                <form hx-post="/secrets/add" hx-target="#secrets-list" hx-swap="outerHTML">
                    <div class="form-group">
                        <label><strong>Secret Key:</strong></label>
                        <input type="text" name="key" required placeholder="e.g., DATABASE_URL or MYAPP_API_KEY" autocomplete="off">
                        <small>Use PROJECT_NAME_ prefix for project-specific secrets</small>
                    </div>
                    <div class="form-group">
                        <label><strong>Secret Value:</strong></label>
                        <input type="password" name="value" required placeholder="Sensitive data (will be encrypted)" autocomplete="off">
                    </div>
                    <div class="form-group">
                        <label><strong>Description:</strong></label>
                        <input type="text" name="description" placeholder="Optional description of this secret">
                    </div>
                    <button type="submit" class="btn btn-success">🔐 Add Secret</button>
                </form>
            </div>
        </div>

        <div class="card">
            <h2>Current Secrets</h2>
            <div id="secrets-list">
                {{template "secrets-table" .}}
            </div>
        </div>
    </div>
</body>
</html>

{{define "secrets-table"}}
<table>
    <thead>
        <tr>
            <th>Key</th>
            <th>Description</th>
            <th>Created</th>
            <th>Updated</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        {{range $key, $secret := .}}
        <tr>
            <td><code>{{$secret.Key}}</code></td>
            <td>{{$secret.Description}}</td>
            <td>{{$secret.CreatedAt}}</td>
            <td>{{$secret.UpdatedAt}}</td>
            <td>
                <button class="btn btn-danger" 
                        hx-post="/secrets/delete" 
                        hx-vals='{"key": "{{$secret.Key}}"}'
                        hx-target="#secrets-list"
                        hx-confirm="Are you sure you want to delete the secret '{{$secret.Key}}'?"
                        hx-swap="outerHTML">Delete</button>
            </td>
        </tr>
        {{end}}
    </tbody>
</table>
{{if eq (len .) 0}}
<p>No secrets configured yet. Add your first secret using the form above.</p>
{{end}}
{{end}}
`
		
		secrets := secretsManager.ListSecrets()
		t, _ := template.New("secrets").Parse(tmpl)
		t.Execute(w, secrets)
	}
}

func addSecretHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.FormValue("key")
	value := r.FormValue("value")
	description := r.FormValue("description")

	if key == "" || value == "" {
		http.Error(w, "Key and value are required", http.StatusBadRequest)
		return
	}

	// Validate key format
	if strings.ContainsAny(key, " \t\n\r") {
		http.Error(w, "Secret key cannot contain whitespace", http.StatusBadRequest)
		return
	}

	err := secretsManager.AddSecret(key, value, description)
	if err != nil {
		log.Printf("Error adding secret %s: %v", key, err)
		http.Error(w, "Failed to add secret", http.StatusInternalServerError)
		return
	}

	log.Printf("Added secret: %s", key)

	// Return updated secrets table
	tmpl := `
<table>
    <thead>
        <tr>
            <th>Key</th>
            <th>Description</th>
            <th>Created</th>
            <th>Updated</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        {{range $key, $secret := .}}
        <tr>
            <td><code>{{$secret.Key}}</code></td>
            <td>{{$secret.Description}}</td>
            <td>{{$secret.CreatedAt}}</td>
            <td>{{$secret.UpdatedAt}}</td>
            <td>
                <button class="btn btn-danger" 
                        hx-post="/secrets/delete" 
                        hx-vals='{"key": "{{$secret.Key}}"}'
                        hx-target="#secrets-list"
                        hx-confirm="Are you sure you want to delete the secret '{{$secret.Key}}'?"
                        hx-swap="outerHTML">Delete</button>
            </td>
        </tr>
        {{end}}
    </tbody>
</table>
{{if eq (len .) 0}}
<p>No secrets configured yet. Add your first secret using the form above.</p>
{{end}}
`

	secrets := secretsManager.ListSecrets()
	t, _ := template.New("secrets-table").Parse(tmpl)
	t.Execute(w, secrets)
}

func deleteSecretHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.FormValue("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	err := secretsManager.DeleteSecret(key)
	if err != nil {
		log.Printf("Error deleting secret %s: %v", key, err)
		http.Error(w, "Failed to delete secret", http.StatusInternalServerError)
		return
	}

	log.Printf("Deleted secret: %s", key)

	// Return updated secrets table
	tmpl := `
<table>
    <thead>
        <tr>
            <th>Key</th>
            <th>Description</th>
            <th>Created</th>
            <th>Updated</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        {{range $key, $secret := .}}
        <tr>
            <td><code>{{$secret.Key}}</code></td>
            <td>{{$secret.Description}}</td>
            <td>{{$secret.CreatedAt}}</td>
            <td>{{$secret.UpdatedAt}}</td>
            <td>
                <button class="btn btn-danger" 
                        hx-post="/secrets/delete" 
                        hx-vals='{"key": "{{$secret.Key}}"}'
                        hx-target="#secrets-list"
                        hx-confirm="Are you sure you want to delete the secret '{{$secret.Key}}'?"
                        hx-swap="outerHTML">Delete</button>
            </td>
        </tr>
        {{end}}
    </tbody>
</table>
{{if eq (len .) 0}}
<p>No secrets configured yet. Add your first secret using the form above.</p>
{{end}}
`

	secrets := secretsManager.ListSecrets()
	t, _ := template.New("secrets-table").Parse(tmpl)
	t.Execute(w, secrets)
}