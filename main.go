package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	Query string `json:"query"`
	Mode  string `json:"mode"` // "sequential" or "parallel"
}

type ThoughtResponse struct {
	ThoughtCloud []string           `json:"thought_cloud"`
	TaskResults  []tasks.TaskResult `json:"task_results"`
	Conclusion   string            `json:"conclusion"`
}

func setupLogging() (*log.Logger, error) {
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	return log.New(multiWriter, "[ThoughtChain] ", log.LstdFlags|log.Lshortfile), nil
}

func setupTaskMapper() *tasks.TaskMapper {
	mapper := tasks.NewTaskMapper()

	// Register file operation patterns
	fileOpsGroup := &tasks.TaskGroup{
		Name:         "file-ops",
		Description:  "File system operations",
		ExecutionMode: tasks.Sequential,
		Patterns: []*tasks.TaskPattern{
			{
				Pattern:   `create directory (\w+)`,
				Variables: []string{"dirname"},
				Template: tasks.TaskDefinition{
					Type:      tasks.FileSystemTask,
					Operation: tasks.WriteOperation,
					Path:      "${dirname}",
				},
			},
			{
				Pattern:   `write file (\w+\.\w+) with content (.+)`,
				Variables: []string{"filename", "content"},
				Template: tasks.TaskDefinition{
					Type:      tasks.FileSystemTask,
					Operation: tasks.WriteOperation,
					Path:      "${filename}",
					Body:      []byte("${content}"),
				},
			},
			{
				Pattern:   `delete file (\w+\.\w+)`,
				Variables: []string{"filename"},
				Template: tasks.TaskDefinition{
					Type:      tasks.FileSystemTask,
					Operation: tasks.DeleteOperation,
					Path:      "${filename}",
				},
			},
		},
	}

	// Register API operation patterns
	apiOpsGroup := &tasks.TaskGroup{
		Name:         "api-ops",
		Description:  "API operations",
		ExecutionMode: tasks.Parallel,
		Patterns: []*tasks.TaskPattern{
			{
				Pattern:   `make GET request to (https?://\S+)`,
				Variables: []string{"url"},
				Template: tasks.TaskDefinition{
					Type:   tasks.APITask,
					Method: "GET",
					URL:    "${url}",
					Headers: map[string]string{
						"Accept": "application/json",
					},
				},
			},
			{
				Pattern:   `post to (https?://\S+) with data (.+)`,
				Variables: []string{"url", "data"},
				Template: tasks.TaskDefinition{
					Type:   tasks.APITask,
					Method: "POST",
					URL:    "${url}",
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					Body: []byte("${data}"),
				},
			},
		},
	}

	mapper.RegisterTaskGroup(fileOpsGroup)
	mapper.RegisterTaskGroup(apiOpsGroup)

	return mapper
}

func setupAgents(logger *log.Logger) (*agent.AgentManager, error) {
	// Create agent manager
	manager := agent.NewAgentManager(configDir, logger)

	// Load agent configurations
	if err := manager.LoadAgents(); err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
	}

	logger.Printf("Loaded agent configurations")
	return manager, nil
}

func setupServer(logger *log.Logger) (llms.LLMClient, *agent.AgentManager, *tasks.TaskMapper, error) {
	logger.Println("Setting up server...")

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, nil, nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
	}
	logger.Println("Found OpenRouter API key")

	openRouterConfig := llms.OpenRouterConfig{
		APIKey:  apiKey,
		BaseURL: "https://openrouter.ai/api/v1/chat/completions",
	}

	llmClient := llms.NewOpenRouterClient(openRouterConfig, logger)
	logger.Println("Initialized LLM client")

	agentManager, err := setupAgents(logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to setup agents: %w", err)
	}

	taskMapper := setupTaskMapper()
	logger.Println("Initialized task mapper")

	return llmClient, agentManager, taskMapper, nil
}

func handleThoughtChain(llmClient llms.LLMClient, taskMapper *tasks.TaskMapper, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ThoughtRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Generate initial thoughts
		thoughts, err := llmClient.GenerateThoughts(ctx, req.Query)
		if err != nil {
			logger.Printf("Failed to generate thoughts: %v", err)
			http.Error(w, "Failed to generate thoughts", http.StatusInternalServerError)
			return
		}

		// Map thoughts to tasks
		chain, err := taskMapper.MapPromptToTasks(ctx, req.Query)
		if err != nil {
			logger.Printf("Failed to map thoughts to tasks: %v", err)
			http.Error(w, "Failed to map thoughts to tasks", http.StatusInternalServerError)
			return
		}

		// Set execution mode based on request
		if req.Mode == "parallel" {
			chain.ExecutionMode = tasks.Parallel
		}

		// Execute tasks
		err = chain.Execute(ctx)
		if err != nil {
			logger.Printf("Failed to execute tasks: %v", err)
			http.Error(w, "Failed to execute tasks", http.StatusInternalServerError)
			return
		}

		// Generate conclusion
		conclusion, err := llmClient.ConcludeThoughts(ctx, thoughts)
		if err != nil {
			logger.Printf("Failed to generate conclusion: %v", err)
			http.Error(w, "Failed to generate conclusion", http.StatusInternalServerError)
			return
		}

		// Prepare response
		resp := ThoughtResponse{
			ThoughtCloud: thoughts,
			TaskResults:  make([]tasks.TaskResult, 0),
			Conclusion:   conclusion,
		}

		// Add task results
		for id, task := range chain.Tasks {
			resp.TaskResults = append(resp.TaskResults, tasks.TaskResult{
				ID:          id,
				Description: task.Description(),
				Success:     true,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func main() {
	logger, err := setupLogging()
	if err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	logger.Println("Logging initialized")

	llmClient, agentManager, taskMapper, err := setupServer(logger)
	if err != nil {
		logger.Fatalf("Failed to setup server: %v", err)
	}

	// Add custom context for file operations
	agentManager.StateManager.States["file_agent"].CurrentContext["working_directory"] = "."
	agentManager.StateManager.States["default_agent"].CurrentContext["working_directory"] = "."

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

	loggedHandler("/thoughtchain", handleThoughtChain(llmClient, taskMapper, logger))

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	logger.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		logger.Fatalf("Server failed to start: %v", err)
	}
}
