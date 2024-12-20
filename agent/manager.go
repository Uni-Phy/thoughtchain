package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadAgents loads agent configurations from the config directory
func (am *AgentManager) LoadAgents() error {
	files, err := filepath.Glob(filepath.Join(am.ConfigDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list config files: %w", err)
	}

	for _, file := range files {
		var agent Agent
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read config file %s: %w", file, err)
		}

		if err := json.Unmarshal(data, &agent); err != nil {
			return fmt.Errorf("failed to parse config file %s: %w", file, err)
		}

		am.Agents[agent.Name] = &agent
		am.StateManager.States[agent.Name] = &AgentState{
			CurrentContext: make(map[string]interface{}),
			PromptHistory: make([]string, 0),
			TaskHistory:   make([]string, 0),
			Errors:        make([]error, 0),
		}

		am.Logger.Printf("Loaded agent: %s", agent.Name)
	}

	return nil
}

// GetAgent retrieves an agent by name
func (am *AgentManager) GetAgent(name string) (*Agent, error) {
	agent, exists := am.Agents[name]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", name)
	}
	return agent, nil
}

// UpdateAgentState updates the state of an agent
func (am *AgentManager) UpdateAgentState(name string, context map[string]interface{}, prompt string, task string, err error) {
	state := am.StateManager.States[name]
	if state == nil {
		state = &AgentState{
			CurrentContext: make(map[string]interface{}),
			PromptHistory: make([]string, 0),
			TaskHistory:   make([]string, 0),
			Errors:        make([]error, 0),
		}
		am.StateManager.States[name] = state
	}

	// Update context
	for k, v := range context {
		state.CurrentContext[k] = v
	}

	// Update histories
	if prompt != "" {
		state.PromptHistory = append(state.PromptHistory, prompt)
	}
	if task != "" {
		state.TaskHistory = append(state.TaskHistory, task)
	}
	if err != nil {
		state.Errors = append(state.Errors, err)
	}
}

// GetAgentState retrieves the state of an agent
func (am *AgentManager) GetAgentState(name string) (*AgentState, error) {
	state, exists := am.StateManager.States[name]
	if !exists {
		return nil, fmt.Errorf("state for agent %s not found", name)
	}
	return state, nil
}
