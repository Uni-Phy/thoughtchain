package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupTestAgents(t *testing.T) (*AgentManager, string) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
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

	// Initialize manager
	manager := NewAgentManager(tmpDir)
	if err := manager.LoadAgents(); err != nil {
		t.Fatal(err)
	}

	return manager, tmpDir
}

func TestAgentManager(t *testing.T) {
	manager, tmpDir := setupTestAgents(t)
	defer os.RemoveAll(tmpDir)

	t.Run("Load agents", func(t *testing.T) {
		if len(manager.Agents) != 1 {
			t.Errorf("Expected 1 agent, got %d", len(manager.Agents))
		}

		agent := manager.Agents["test_agent"]
		if agent == nil {
			t.Error("Expected test_agent to exist")
		}
		if agent != nil && agent.Name != "test_agent" {
			t.Errorf("Expected agent name 'test_agent', got %s", agent.Name)
		}
	})

	t.Run("Get agent", func(t *testing.T) {
		agent, err := manager.GetAgent("test_agent")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if agent == nil {
			t.Error("Expected agent to be returned")
		}
	})

	t.Run("Agent state management", func(t *testing.T) {
		context := map[string]interface{}{
			"test_key": "test_value",
		}
		manager.UpdateAgentState("test_agent", context, "test prompt", "test task", nil)

		state, err := manager.GetAgentState("test_agent")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if state == nil {
			t.Error("Expected state to be returned")
		}
		if val, ok := state.CurrentContext["test_key"]; !ok || val != "test_value" {
			t.Error("Expected context to be updated")
		}
	})
}

func TestPromptConfigurator(t *testing.T) {
	manager, tmpDir := setupTestAgents(t)
	defer os.RemoveAll(tmpDir)

	configurator := NewAdvancedPromptConfigurator(manager)

	t.Run("Configure prompt", func(t *testing.T) {
		agent, _ := manager.GetAgent("test_agent")
		prompt, err := configurator.ConfigurePrompt(context.Background(), agent, "test task")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		expected := "Process the following task: test task"
		if prompt != expected {
			t.Errorf("Expected prompt %q, got %q", expected, prompt)
		}
	})
}

func TestAgentSelector(t *testing.T) {
	manager, tmpDir := setupTestAgents(t)
	defer os.RemoveAll(tmpDir)

	selector := NewSemanticAgentSelector(manager, 0.5)

	t.Run("Select agent", func(t *testing.T) {
		agent, err := selector.SelectAgent(context.Background(), "perform file operations")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if agent == nil {
			t.Error("Expected agent to be returned")
		}
		if agent.Name != "test_agent" {
			t.Errorf("Expected test_agent, got %s", agent.Name)
		}
	})
}
