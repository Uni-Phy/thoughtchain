package llms

import (
	"encoding/json"
	"fmt"
	"net/http"
	"thoughtchain/tasks"
)

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

func HandleThoughtChain(llmClient LLMClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

			// Generate a unique task ID
			taskID := fmt.Sprintf("task-%d", i+1)
			
			// Add task to chain with ID and dependencies
			taskChain.AddTask(taskID, t, taskDef.DependsOn)
			sequence = append(sequence, t.Description())

			// Execute each task individually to track results
			err = t.Execute(ctx)
			result := tasks.TaskResult{
				ID:          taskID,
				Description: t.Description(),
				Success:     err == nil,
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
}
