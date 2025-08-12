package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type APIResponse struct {
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

type TaskManager struct {
	tasks map[string]string
	mutex sync.RWMutex
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]string),
	}
}

func (tm *TaskManager) AddTask(id, description string) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.tasks[id] = description
}

func (tm *TaskManager) GetTasks() map[string]string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	result := make(map[string]string)
	for k, v := range tm.tasks {
		result[k] = v
	}
	return result
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	taskManager := NewTaskManager()

	// Add some demo tasks
	taskManager.AddTask("1", "Deploy Hello World app")
	taskManager.AddTask("2", "Set up Git repository")
	taskManager.AddTask("3", "Configure secrets management")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := APIResponse{
			Message:   "API Server Demo - Running via Draheim",
			Timestamp: time.Now().Format(time.RFC3339),
			Data: map[string]interface{}{
				"port":    port,
				"version": "1.0.0",
				"status":  "healthy",
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := APIResponse{
			Message:   "Task list retrieved successfully",
			Timestamp: time.Now().Format(time.RFC3339),
			Data: map[string]interface{}{
				"tasks": taskManager.GetTasks(),
				"count": len(taskManager.GetTasks()),
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := APIResponse{
			Message:   "Service is healthy",
			Timestamp: time.Now().Format(time.RFC3339),
			Data: map[string]interface{}{
				"uptime": "running",
				"port":   port,
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	log.Printf("API Server demo starting on port %s", port)
	log.Printf("Available endpoints:")
	log.Printf("  GET /        - API info")
	log.Printf("  GET /tasks   - Task list")
	log.Printf("  GET /health  - Health check")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
