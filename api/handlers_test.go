package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"thoughtchain/agent"
	"thoughtchain/tasks"
)

// MockLLMClient implements the llms.LLMClient interface for testing
type MockLLMClient struct{}

func (m *MockLLMClient) GenerateThoughts(ctx context.Context, query string) ([]string, error) {
	return []string{"Test thought"}, nil
}

func (m *MockLLMClient) ConcludeThoughts(ctx context.Context, thoughts []string) (string, error) {
	return "Test conclusion", nil
}

func setupTestServer(t *testing.T) (*Server, string) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "api-test-*")
	if err != nil {
		t.Fatal(err)
	}

	// Create a simple test agent config
	testConfig := []byte(`{
		"name": "test_agent",
		"description": "Test agent for file operations",
		"prompt_templates": {
			"default": "Process the following task: {{.Task}}"
		},
		"capabilities": ["file_operations"],
		"required_context": [],
		"chains_with": [],
		"max_tokens": 1000,
		"temperature": 0.7
	}`)

	// Write test config
	configPath := filepath.Join(tmpDir, "test_agent.json")
	if err := os.WriteFile(configPath, testConfig, 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize components
	agentManager := agent.NewAgentManager(tmpDir)
	if err := agentManager.LoadAgents(); err != nil {
		t.Fatal(err)
	}

	agentSelector := agent.NewSemanticAgentSelector(agentManager, 0.5)
	promptConfigurator := agent.NewAdvancedPromptConfigurator(agentManager)
	logger := log.New(os.Stdout, "TEST: ", log.LstdFlags)

	// Create server
	server := NewServer(
		&MockLLMClient{},
		agentManager,
		agentSelector,
		promptConfigurator,
		logger,
	)

	return server, tmpDir
}

func TestHandleThoughtChain(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	t.Run("Successful request", func(t *testing.T) {
		requestPayload := ThoughtRequest{
			Query: "perform file operations",
			Tasks: []tasks.TaskDefinition{
				{
					Type:      tasks.FileSystemTask,
					Operation: tasks.WriteOperation,
					Path:      filepath.Join(tmpDir, "test.txt"),
					Body:      []byte("test content"),
				},
			},
		}

		requestBody, _ := json.Marshal(requestPayload)
		req := httptest.NewRequest(http.MethodPost, "/thoughtchain", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		server.HandleThoughtChain(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", resp.Code)
		}

		var response ThoughtResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if len(response.ThoughtCloud) == 0 {
			t.Error("Expected non-empty thought cloud")
		}
		if response.Conclusion == "" {
			t.Error("Expected non-empty conclusion")
		}
	})

	t.Run("Invalid request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/thoughtchain", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		server.HandleThoughtChain(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("Expected status BadRequest, got %v", resp.Code)
		}
	})
}

func TestHandleHealth(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()

	server.HandleHealth(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", response["status"])
	}
}

func TestHandleListAgents(t *testing.T) {
	server, tmpDir := setupTestServer(t)
	defer os.RemoveAll(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/agents", nil)
	resp := httptest.NewRecorder()

	server.HandleListAgents(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	agents, ok := response["agents"].([]interface{})
	if !ok {
		t.Error("Expected agents array in response")
	}
	if len(agents) == 0 {
		t.Error("Expected non-empty agents list")
	}
}
