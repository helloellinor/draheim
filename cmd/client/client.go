package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type ClientConfig struct {
	ServerURL string `json:"server_url"`
	Username  string `json:"username"`
	APIKey    string `json:"api_key"`
}

type ClientProject struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Port        int    `json:"port"`
	GitRepo     string `json:"git_repo"`
	GitBranch   string `json:"git_branch"`
	WorkingDir  string `json:"working_dir"`
}

func loadClientConfig() *ClientConfig {
	configPath := "draheim-client-config.json"
	
	// Try to load existing config
	if data, err := ioutil.ReadFile(configPath); err == nil {
		var config ClientConfig
		if err := json.Unmarshal(data, &config); err == nil {
			return &config
		}
	}
	
	// Create default config
	config := &ClientConfig{
		ServerURL: "http://localhost:8080",
		Username:  os.Getenv("USER"),
		APIKey:    "",
	}
	
	// Save default config
	if data, err := json.MarshalIndent(config, "", "  "); err == nil {
		ioutil.WriteFile(configPath, data, 0644)
	}
	
	return config
}

func main() {
	config := loadClientConfig()
	
	var (
		action       = flag.String("action", "list", "Action to perform: list, deploy, stop, update, logs, secrets, secret-add, secret-delete")
		name         = flag.String("name", "", "Project name")
		gitRepo      = flag.String("repo", "", "Git repository URL")
		gitBranch    = flag.String("branch", "main", "Git branch")
		buildPath    = flag.String("build-path", "", "sti til main-pakken i repoet, t.d. cmd/tenar")
		port         = flag.Int("port", 8081, "Port number")
		binaryPath   = flag.String("path", "", "Binary path")
		serverURL    = flag.String("server", config.ServerURL, "Server URL")
		secretKey    = flag.String("secret-key", "", "Secret key for secret operations")
		secretValue  = flag.String("secret-value", "", "Secret value for adding secrets")
		secretDesc   = flag.String("secret-desc", "", "Secret description")
	)
	flag.Parse()

	config.ServerURL = *serverURL

	switch *action {
	case "list":
		listProjects(config)
	case "deploy":
		if *name == "" {
			log.Fatal("Project name is required for deploy action")
		}
		deployProject(config, *name, *binaryPath, *gitRepo, *gitBranch, *buildPath, *port)
	case "stop":
		if *name == "" {
			log.Fatal("Project name is required for stop action")
		}
		stopProject(config, *name)
	case "update":
		if *name == "" {
			log.Fatal("Project name is required for update action")
		}
		updateProject(config, *name)
	case "logs":
		if *name == "" {
			log.Fatal("Project name is required for logs action")
		}
		showLogs(config, *name)
	case "secrets":
		listSecrets(config)
	case "secret-add":
		if *secretKey == "" || *secretValue == "" {
			log.Fatal("Secret key and value are required for secret-add action")
		}
		addSecret(config, *secretKey, *secretValue, *secretDesc)
	case "secret-delete":
		if *secretKey == "" {
			log.Fatal("Secret key is required for secret-delete action")
		}
		deleteSecret(config, *secretKey)
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		fmt.Println("Available actions: list, deploy, stop, update, logs, secrets, secret-add, secret-delete")
		os.Exit(1)
	}
}

// hent og send legg API-nøkkelen paa kvar førespurnad. Klienten har hatt
// api_key i konfigurasjonen sin heile tida, men sende han aldri - og
// tenaren las han aldri - so alt stod ope for kven som helst som naadde
// porten. Utan nøkkel svarar tenaren no 401.
func hent(config *ClientConfig, sti string) (*http.Response, error) {
	req, err := http.NewRequest("GET", config.ServerURL+sti, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	return http.DefaultClient.Do(req)
}

func send(config *ClientConfig, sti string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", config.ServerURL+sti, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	return http.DefaultClient.Do(req)
}

func listProjects(config *ClientConfig) {
	resp, err := hent(config, "/projects")
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Println("Projects on server:")
	fmt.Println(string(body))
}

func deployProject(config *ClientConfig, name, binaryPath, gitRepo, gitBranch, buildPath string, port int) {
	data := url.Values{}
	data.Set("name", name)
	data.Set("path", binaryPath)
	data.Set("git_repo", gitRepo)
	data.Set("build_path", buildPath)
	data.Set("git_branch", gitBranch)
	data.Set("port", fmt.Sprintf("%d", port))

	resp, err := send(config, "/deploy", data)
	if err != nil {
		log.Fatalf("Failed to deploy project: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusSeeOther {
		fmt.Printf("Successfully deployed project: %s\n", name)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Failed to deploy project: %s\n", string(body))
	}
}

func stopProject(config *ClientConfig, name string) {
	data := url.Values{}
	data.Set("name", name)

	resp, err := send(config, "/stop", data)
	if err != nil {
		log.Fatalf("Failed to stop project: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusSeeOther {
		fmt.Printf("Successfully stopped project: %s\n", name)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Failed to stop project: %s\n", string(body))
	}
}

func updateProject(config *ClientConfig, name string) {
	data := url.Values{}
	data.Set("name", name)

	resp, err := send(config, "/update", data)
	if err != nil {
		log.Fatalf("Failed to update project: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusSeeOther {
		fmt.Printf("Successfully updated project: %s\n", name)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Failed to update project: %s\n", string(body))
	}
}

func showLogs(config *ClientConfig, name string) {
	resp, err := hent(config, "/logs?project="+name)
	if err != nil {
		log.Fatalf("Failed to get logs: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read logs: %v", err)
	}

	// Extract logs from HTML response (simple approach)
	content := string(body)
	if start := strings.Index(content, "<div class=\"logs\">"); start != -1 {
		start += len("<div class=\"logs\">")
		if end := strings.Index(content[start:], "</div>"); end != -1 {
			logs := content[start : start+end]
			fmt.Println("Logs for", name+":")
			fmt.Println(logs)
			return
		}
	}
	
	fmt.Println("Could not extract logs from response")
}

func listSecrets(config *ClientConfig) {
	resp, err := hent(config, "/secrets")
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	// Extract secrets table from HTML (simple approach)
	content := string(body)
	fmt.Println("Secrets on server:")
	
	// Look for table content
	if strings.Contains(content, "No secrets configured") {
		fmt.Println("No secrets configured yet.")
		return
	}
	
	// Parse table rows for basic info
	lines := strings.Split(content, "\n")
	fmt.Printf("%-20s %-30s %-20s\n", "KEY", "DESCRIPTION", "CREATED")
	fmt.Println(strings.Repeat("-", 70))
	
	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "<tbody>") {
			inTable = true
			continue
		}
		if strings.Contains(line, "</tbody>") {
			break
		}
		if inTable && strings.Contains(line, "<code>") {
			// Extract key from <code> tags
			start := strings.Index(line, "<code>") + 6
			end := strings.Index(line[start:], "</code>")
			if end > 0 {
				key := line[start : start+end]
				fmt.Printf("%-20s %-30s %-20s\n", key, "[Check web UI]", "[Check web UI]")
			}
		}
	}
}

func addSecret(config *ClientConfig, key, value, description string) {
	data := url.Values{}
	data.Set("key", key)
	data.Set("value", value)
	data.Set("description", description)

	resp, err := send(config, "/secrets/add", data)
	if err != nil {
		log.Fatalf("Failed to add secret: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("Successfully added secret: %s\n", key)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Failed to add secret: %s\n", string(body))
	}
}

func deleteSecret(config *ClientConfig, key string) {
	data := url.Values{}
	data.Set("key", key)

	resp, err := send(config, "/secrets/delete", data)
	if err != nil {
		log.Fatalf("Failed to delete secret: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("Successfully deleted secret: %s\n", key)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Failed to delete secret: %s\n", string(body))
	}
}