package main

import (
	"testing"
	"time"
)

func TestProjectManager(t *testing.T) {
	pm := NewProjectManager()
	
	// Test that new ProjectManager is empty
	if len(pm.GetProjects()) != 0 {
		t.Errorf("Expected empty projects map, got %d projects", len(pm.GetProjects()))
	}
}

func TestProjectCreation(t *testing.T) {
	project := &Project{
		Name:        "test",
		Status:      "running",
		Port:        8080,
		LastUpdated: time.Now(),
		BinaryPath:  "/test/path",
	}
	
	if project.Name != "test" {
		t.Errorf("Expected project name 'test', got '%s'", project.Name)
	}
	
	if project.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", project.Status)
	}
	
	if project.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", project.Port)
	}
}