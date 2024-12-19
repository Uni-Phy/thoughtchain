package agent

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"text/template"
	"os"
)

// PromptConfigurator defines the interface for configuring prompts
type PromptConfigurator interface {
	ConfigurePrompt(ctx context.Context, agent *Agent, taskDescription string) (string, error)
}

// TemplateData holds the data available to prompt templates
type TemplateData struct {
	Task           string
	AgentName      string
	AgentDesc      string
	Context        map[string]interface{}
	PreviousTasks  []string
	PreviousErrors []string
}

// AdvancedPromptConfigurator implements sophisticated prompt configuration
type AdvancedPromptConfigurator struct {
	AgentManager *AgentManager
	Logger *log.Logger
}

func NewAdvancedPromptConfigurator(agentManager *AgentManager) *AdvancedPromptConfigurator {

	logger :=  log.New(os.Stdout, "[PROMPT] ", log.LstdFlags|log.Lshortfile)
	return &AdvancedPromptConfigurator{
		AgentManager: agentManager,
		Logger: logger,
	}

}

func (p *AdvancedPromptConfigurator) ConfigurePrompt(ctx context.Context, agent *Agent, taskDescription string) (string, error) {
	p.Logger.Printf("Configuring prompt for agent: %s, task: %s", agent.Name, taskDescription)

	// Get agent state for context
	state, err := p.AgentManager.GetAgentState(agent.Name)
	if err != nil {
		p.Logger.Printf("Failed to get agent state: %v", err)
		return "", fmt.Errorf("failed to get agent state: %w", err)
	}


	// Prepare template data
	data := TemplateData{
		Task:      taskDescription,
		AgentName: agent.Name,
		AgentDesc: agent.Description,
		Context:   state.CurrentContext,
	}

	// Add history if available
	if state != nil {
		data.PreviousTasks = state.TaskHistory
		for _, err := range state.Errors {
			if err != nil {
				data.PreviousErrors = append(data.PreviousErrors, err.Error())
			}
		}
	}

	// Get the appropriate template
	templateContent, ok := agent.PromptTemplates["default"]
	if !ok {
		p.Logger.Printf("No default template found for agent: %s", agent.Name)
		return "", fmt.Errorf("no default template found for agent %s", agent.Name)
	}

	// Parse and execute the template
	tmpl, err := template.New("prompt").Funcs(p.getTemplateFuncs()).Parse(templateContent)
	if err != nil {
		p.Logger.Printf("Failed to parse template: %v", err)
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		p.Logger.Printf("Failed to execute template: %v", err)
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	prompt := buf.String()
	p.Logger.Printf("Generated prompt: %s", prompt)

	// Validate the generated prompt
	if err := p.validatePrompt(prompt, agent); err != nil {
		p.Logger.Printf("Prompt validation failed: %v", err)
		return "", fmt.Errorf("prompt validation failed: %w", err)
	}

	// Update agent state with the new prompt
	p.AgentManager.UpdateAgentState(agent.Name, nil, prompt, taskDescription, nil)

	p.Logger.Printf("Successfully configured prompt for agent: %s", agent.Name)
	return prompt, nil
}

func (p *AdvancedPromptConfigurator) getTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"join": func(sep string, items []string) string {
			var result string
			for i, item := range items {
				if i > 0 {
					result += sep
				}
				result += item
			}
			return result
		},
		"contextValue": func(key string, context map[string]interface{}) interface{} {
			if val, ok := context[key]; ok {
				return val
			}
			return nil
		},
		"hasContext": func(key string, context map[string]interface{}) bool {
			_, ok := context[key]
			return ok
		},
		"lastError": func(errors []string) string {
			if len(errors) > 0 {
				return errors[len(errors)-1]
			}
			return ""
		},
		"lastTask": func(tasks []string) string {
			if len(tasks) > 0 {
				return tasks[len(tasks)-1]
			}
			return ""
		},
	}
}

func (p *AdvancedPromptConfigurator) validatePrompt(prompt string, agent *Agent) error {
	// Basic validation
	if len(prompt) == 0 {
		p.Logger.Println("Prompt validation failed: empty prompt generated")
		return fmt.Errorf("empty prompt generated")
	}

	// Check if prompt is within token limit
	// This is a simplified check - in practice you would use a proper tokenizer
	if len(prompt) > agent.MaxTokens*4 {
		p.Logger.Println("Prompt validation failed: prompt exceeds maximum token limit")
		return fmt.Errorf("prompt exceeds maximum token limit")
	}

	// Check if prompt contains required context
	for _, required := range agent.RequiredContext {
		if !bytes.Contains([]byte(prompt), []byte(required)) {
			p.Logger.Printf("Prompt validation failed: prompt missing required context %s", required)
			return fmt.Errorf("prompt missing required context: %s", required)
		}
	}

	return nil
}