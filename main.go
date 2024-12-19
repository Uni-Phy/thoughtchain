package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"thoughtchain/agent"
	"thoughtchain/llms"
	"thoughtchain/tasks"
)

const (
	defaultPort = ":8080"
	configDir   = "config/agents"
	logFile     = "thoughtchain.log"
)

// Define the structs for thought request and response
type ThoughtRequest struct {
	Query string              `json:"query"`
	Tasks []tasks.TaskDefinition `json:"tasks"`
}

type ThoughtResponse struct {
	ThoughtCloud []string           `json:"thought_cloud"`
	Sequence     []string           `json:"sequence"`
	TaskResults  []tasks.TaskResult `json:"task_results"`
	Conclusion   string             `json:"conclusion"`
}

func setupLogging() (*log.Logger, error) {
	// Create log file
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer for both file and console
	multiWriter := io.MultiWriter(os.Stdout, file)

	// Create logger with file name and line number
	return log.New(multiWriter, "[ThoughtChain] ", log.LstdFlags|log.Lshortfile), nil
}

func setupAgents(logger *log.Logger) (*agent.AgentManager, error) {
	logger.Println("Setting up agents...")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}
	logger.Printf("Created config directory: %s", configDir)

	// Write example agent configurations
	for filename, content := range agent.ExampleAgentConfigs {
		path := filepath.Join(configDir, filename)
		if err := os.WriteFile(path, content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write config file %s: %w", filename, err)
		}
		logger.Printf("Created agent config: %s", filename)
	}

	// Initialize agent manager
	agentManager := agent.NewAgentManager(configDir)
	if err := agentManager.LoadAgents(); err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
	}
	logger.Printf("Loaded %d agents", len(agentManager.Agents))

	return agentManager, nil
}

func setupServer(logger *log.Logger) (llms.LLMClient, *agent.AgentManager, error) {
	logger.Println("Setting up server...")

	// Get OpenRouter API key
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
	}
	logger.Println("Found OpenRouter API key")

	// Initialize LLM client
	openRouterConfig := llms.OpenRouterConfig{
		APIKey:  apiKey,
		BaseURL: "https://openrouter.ai/api/v1/chat/completions",
	}

	llmClient := llms.NewOpenRouterClient(openRouterConfig, logger)
	logger.Println("Initialized LLM client")

	// Setup agents
	agentManager, err := setupAgents(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup agents: %w", err)
	}

	return llmClient, agentManager, nil
}

func main() {
	// Setup logging
	logger, err := setupLogging()
	if err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	logger.Println("Logging initialized")

	// Setup server
	llmClient, agentManager, err := setupServer(logger)
	if err != nil {
		logger.Fatalf("Failed to setup server: %v", err)
	}

	// Add custom context for file operations to ensure it is always available
	agentManager.StateManager.States["file_agent"].CurrentContext["working_directory"] = "."
	agentManager.StateManager.States["default_agent"].CurrentContext["working_directory"] = "."

	// Setup routes
	logger.Println("Setting up routes...")

	// Wrap handlers with logging middleware
	loggedHandler := func(path string, handler http.HandlerFunc) {
		http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			logger.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
			handler(w, r)
		})
		logger.Printf("Registered route: %s", path)
	}

	loggedHandler("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	loggedHandler("/agents", func(w http.ResponseWriter, r *http.Request) {
		agents := agentManager.StateManager.ListAgents()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents)
	})

	// Create the thoughtchain handler directly in main.go
	thoughtChainHandler := func(w http.ResponseWriter, r *http.Request) {
		var req ThoughtRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Generate thoughts about the task execution
		thoughts, err := llmClient.GenerateThoughts(ctx, req.Query)
		if err != nil {
			http.Error(w, "Failed to generate thoughts", http.StatusInternalServerError)
			return
		}

		// Create and execute task chain
		taskChain := tasks.NewTaskChain()
		var taskResults []tasks.TaskResult
		var sequence []string

		for i, taskDef := range req.Tasks {
			t, err := tasks.CreateTask(taskDef)
			if err != nil {
				http.Error(w, "Failed to create task", http.StatusBadRequest)
				return
			}

			taskChain.AddTask(t)
			sequence = append(sequence, t.Description())

			// Execute each task individually to track results
			err = t.Execute(ctx)
			result := tasks.TaskResult{
				Description: t.Description(),
				Success:    err == nil,
			}
			if err != nil {
				result.Error = err.Error()
				thoughts = append(thoughts, fmt.Sprintf("Task %d failed: %s", i+1, err.Error()))
			} else {
				thoughts = append(thoughts, fmt.Sprintf("Successfully executed task %d", i+1))
			}
			taskResults = append(taskResults, result)
		}

		// Generate conclusion based on execution results
		conclusion, err := llmClient.ConcludeThoughts(ctx, thoughts)
		if err != nil {
			http.Error(w, "Failed to conclude thoughts", http.StatusInternalServerError)
			return
		}

		resp := ThoughtResponse{
			ThoughtCloud: thoughts,
			Sequence:     sequence,
			TaskResults:  taskResults,
			Conclusion:   conclusion,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	loggedHandler("/thoughtchain", thoughtChainHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	logger.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		logger.Fatalf("Server failed to start: %v", err)
	}
}