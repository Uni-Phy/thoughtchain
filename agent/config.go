package agent

import "log"

// AgentState tracks the current state and history of an agent
type AgentState struct {
	CurrentContext map[string]interface{}
	PromptHistory  []string
	TaskHistory    []string
	Errors         []error
}

// AgentManager handles loading and managing agents
type AgentManager struct {
	Agents       map[string]*Agent
	ConfigDir    string
	StateManager *StateManager
	Logger       *log.Logger
}

// StateManager handles agent state persistence
type StateManager struct {
	States map[string]*AgentState
}

// ListAgents returns a list of all agent names
func (sm *StateManager) ListAgents() []string {
	agents := []string{}
	for name := range sm.States {
		agents = append(agents, name)
	}
	return agents
}

// NewAgentManager creates a new agent manager
func NewAgentManager(configDir string, logger *log.Logger) *AgentManager {
	return &AgentManager{
		Agents:    make(map[string]*Agent),
		ConfigDir: configDir,
		StateManager: &StateManager{
			States: make(map[string]*AgentState),
		},
		Logger: logger,
	}
}
