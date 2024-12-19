package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"thoughtchain/agent"
	"thoughtchain/llms"
	"thoughtchain/tasks"
)

// Server represents the API server
type Server struct {
	LLMClient         llms.LLMClient
	AgentManager      *agent.AgentManager
	AgentSelector     agent.AgentSelector
	PromptConfigurator agent.PromptConfigurator
	Logger            *log.Logger
}

// NewServer creates a new API server
func NewServer(
	llmClient llms.LLMClient,
	agentManager *agent.AgentManager,
	agentSelector agent.AgentSelector,
	promptConfigurator agent.PromptConfigurator,
	logger *log.Logger,
) *Server {
	return &Server{
		LLMClient:         llmClient,
		AgentManager:      agentManager,
		AgentSelector:     agentSelector,
		PromptConfigurator: promptConfigurator,
		Logger:            logger,
	}
}

// ThoughtRequest represents an incoming request
type ThoughtRequest struct {
	Query string              `json:"query"`
	Tasks []tasks.TaskDefinition `json:"tasks"`
}

// ThoughtResponse represents the response to a thought chain request
type ThoughtResponse struct {
	ThoughtCloud []string         `json:"thought_cloud"`
	Sequence     []string         `json:"sequence"`
	TaskResults  []tasks.TaskResult `json:"task_results"`
	Conclusion   string           `json:"conclusion"`
}

// HandleThoughtChain handles thought chain requests
func (s *Server) HandleThoughtChain(w http.ResponseWriter, r *http.Request) {
	var req ThoughtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.handleError(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Select appropriate agent
	selectedAgent, err := s.AgentSelector.SelectAgent(ctx, req.Query)
	if err != nil {
		s.handleError(w, "Failed to select agent", http.StatusInternalServerError)
		return
	}

	// Configure prompt
	configuredPrompt, err := s.PromptConfigurator.ConfigurePrompt(ctx, selectedAgent, req.Query)
	if err != nil {
		s.handleError(w, "Failed to configure prompt", http.StatusInternalServerError)
		return
	}

	// Generate thoughts
	thoughts, err := s.LLMClient.GenerateThoughts(ctx, configuredPrompt)
	if err != nil {
		s.handleError(w, "Failed to generate thoughts", http.StatusInternalServerError)
		return
	}

	// Execute tasks
	taskResults, sequence := s.executeTasks(ctx, req.Tasks, thoughts)

	// Generate conclusion
	conclusion, err := s.LLMClient.ConcludeThoughts(ctx, thoughts)
	if err != nil {
		s.handleError(w, "Failed to conclude thoughts", http.StatusInternalServerError)
		return
	}

	resp := ThoughtResponse{
		ThoughtCloud: thoughts,
		Sequence:     sequence,
		TaskResults:  taskResults,
		Conclusion:   conclusion,
	}

	s.sendJSON(w, resp)
}

// HandleHealth handles health check requests
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendJSON(w, map[string]string{"status": "healthy"})
}

// HandleListAgents handles requests to list available agents
func (s *Server) HandleListAgents(w http.ResponseWriter, r *http.Request) {
	agents := make([]map[string]interface{}, 0)
	for name, agent := range s.AgentManager.Agents {
		agents = append(agents, map[string]interface{}{
			"name":         name,
			"description": agent.Description,
			"capabilities": agent.Capabilities,
		})
	}
	s.sendJSON(w, map[string]interface{}{"agents": agents})
}

// executeTasks executes a sequence of tasks and returns results
func (s *Server) executeTasks(ctx context.Context, taskDefs []tasks.TaskDefinition, thoughts []string) ([]tasks.TaskResult, []string) {
	var taskResults []tasks.TaskResult
	var sequence []string

	taskChain := tasks.NewTaskChain()

	for i, taskDef := range taskDefs {
		task, err := tasks.CreateTask(taskDef)
		if err != nil {
			taskResults = append(taskResults, tasks.TaskResult{
				Description: fmt.Sprintf("Failed to create task %d", i+1),
				Success:    false,
				Error:      err.Error(),
			})
			continue
		}

		taskChain.AddTask(task)
		sequence = append(sequence, task.Description())

		err = task.Execute(ctx)
		result := tasks.TaskResult{
			Description: task.Description(),
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

	return taskResults, sequence
}

// handleError sends an error response
func (s *Server) handleError(w http.ResponseWriter, message string, status int) {
	s.Logger.Printf("Error: %s (Status: %d)", message, status)
	http.Error(w, message, status)
}

// sendJSON sends a JSON response
func (s *Server) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.handleError(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
