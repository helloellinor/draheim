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
		action     = flag.String("action", "list", "Action to perform: list, deploy, stop, update, logs")
		name       = flag.String("name", "", "Project name")
		gitRepo    = flag.String("repo", "", "Git repository URL")
		gitBranch  = flag.String("branch", "main", "Git branch")
		port       = flag.Int("port", 8081, "Port number")
		binaryPath = flag.String("path", "", "Binary path")
		serverURL  = flag.String("server", config.ServerURL, "Server URL")
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
		deployProject(config, *name, *binaryPath, *gitRepo, *gitBranch, *port)
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
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		fmt.Println("Available actions: list, deploy, stop, update, logs")
		os.Exit(1)
	}
}

func listProjects(config *ClientConfig) {
	resp, err := http.Get(config.ServerURL + "/projects")
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

func deployProject(config *ClientConfig, name, binaryPath, gitRepo, gitBranch string, port int) {
	data := url.Values{}
	data.Set("name", name)
	data.Set("path", binaryPath)
	data.Set("git_repo", gitRepo)
	data.Set("git_branch", gitBranch)
	data.Set("port", fmt.Sprintf("%d", port))

	resp, err := http.PostForm(config.ServerURL+"/deploy", data)
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

	resp, err := http.PostForm(config.ServerURL+"/stop", data)
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

	resp, err := http.PostForm(config.ServerURL+"/update", data)
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
	resp, err := http.Get(config.ServerURL + "/logs?project=" + name)
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