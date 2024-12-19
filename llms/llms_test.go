package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"thoughtchain/tasks"
)

func setupTestLogger() *log.Logger {
	return log.New(ioutil.Discard, "", 0) // Silent logger for tests
}

func TestOpenRouterClient(t *testing.T) {
	logger := setupTestLogger()

	t.Run("GenerateThoughts", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("Expected POST request, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
			}

			response := OpenRouterResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{
						Message: struct {
							Content string `json:"content"`
						}{
							Content: "Test thought",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		// Create client with test server URL
		client := &OpenRouterLLMClient{
			Config: OpenRouterConfig{
				APIKey:  "test-key",
				BaseURL: server.URL,
			},
			HTTPClient: &http.Client{},
			Logger:     logger,
		}

		thoughts, err := client.GenerateThoughts(context.Background(), "test query")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(thoughts) != 1 {
			t.Errorf("Expected 1 thought, got %d", len(thoughts))
		}
		if thoughts[0] != "Test thought" {
			t.Errorf("Expected 'Test thought', got %s", thoughts[0])
		}
	})

	t.Run("ConcludeThoughts", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := OpenRouterResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{
						Message: struct {
							Content string `json:"content"`
						}{
							Content: "Test conclusion",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		// Create client with test server URL
		client := &OpenRouterLLMClient{
			Config: OpenRouterConfig{
				APIKey:  "test-key",
				BaseURL: server.URL,
			},
			HTTPClient: &http.Client{},
			Logger:     logger,
		}

		conclusion, err := client.ConcludeThoughts(context.Background(), []string{"thought1", "thought2"})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if conclusion != "Test conclusion" {
			t.Errorf("Expected 'Test conclusion', got %s", conclusion)
		}
	})

	t.Run("Error handling", func(t *testing.T) {
		// Create test server that returns errors
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		// Create client with test server URL
		client := &OpenRouterLLMClient{
			Config: OpenRouterConfig{
				APIKey:  "test-key",
				BaseURL: server.URL,
			},
			HTTPClient: &http.Client{},
			Logger:     logger,
		}

		_, err := client.GenerateThoughts(context.Background(), "test query")
		if err == nil {
			t.Error("Expected error for server error response")
		}

		_, err = client.ConcludeThoughts(context.Background(), []string{"thought"})
		if err == nil {
			t.Error("Expected error for server error response")
		}
	})
}

func TestHandleThoughtChain(t *testing.T) {
	logger := setupTestLogger()
	client := &OpenRouterLLMClient{
		Config: OpenRouterConfig{
			APIKey:  "test-key",
			BaseURL: "http://test-url",
		},
		HTTPClient: &http.Client{},
		Logger:     logger,
	}
	handler := HandleThoughtChain(client)

	t.Run("Valid request", func(t *testing.T) {
		// Create test server for OpenRouter API
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := OpenRouterResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{
						Message: struct {
							Content string `json:"content"`
						}{
							Content: "Test thought",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client.Config.BaseURL = server.URL

		requestPayload := ThoughtRequest{
			Query: "test query",
			Tasks: []tasks.TaskDefinition{
				{
					Type:      tasks.FileSystemTask,
					Operation: tasks.WriteOperation,
					Path:      "test.txt",
					Body:      []byte("test content"),
				},
			},
		}

		requestBody, _ := json.Marshal(requestPayload)
		req := httptest.NewRequest(http.MethodPost, "/thoughtchain", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		handler(resp, req)

		if resp.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", resp.Code)
		}

		var response ThoughtResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}

		if len(response.ThoughtCloud) == 0 {
			t.Error("Expected non-empty thoughts")
		}
		if response.Conclusion == "" {
			t.Error("Expected non-empty conclusion")
		}
	})

	t.Run("Invalid request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/thoughtchain", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		handler(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Errorf("Expected status BadRequest, got %v", resp.Code)
		}
	})
}
