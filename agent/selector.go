package agent

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// AgentSelector defines the interface for selecting an agent based on a task description
type AgentSelector interface {
	SelectAgent(ctx context.Context, taskDescription string) (*Agent, error)
}

// SimilarityScore represents the similarity between a task and an agent
type SimilarityScore struct {
	Agent *Agent
	Score float64
}

// SemanticAgentSelector selects agents based on semantic similarity
type SemanticAgentSelector struct {
	AgentManager *AgentManager
	MinScore     float64 // Minimum similarity score to consider an agent
}

func NewSemanticAgentSelector(agentManager *AgentManager, minScore float64) *SemanticAgentSelector {
	return &SemanticAgentSelector{
		AgentManager: agentManager,
		MinScore:     minScore,
	}
}

func (s *SemanticAgentSelector) SelectAgent(ctx context.Context, taskDescription string) (*Agent, error) {
	if len(s.AgentManager.Agents) == 0 {
		return nil, fmt.Errorf("no agents available")
	}

	// Calculate similarity scores for all agents
	var scores []SimilarityScore
	for _, agent := range s.AgentManager.Agents {
		score := s.calculateSimilarity(taskDescription, agent)
		scores = append(scores, SimilarityScore{
			Agent: agent,
			Score: score,
		})
	}

	// Sort by score in descending order
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Get the best matching agent(s)
	var selectedAgents []*Agent
	for _, score := range scores {
		if score.Score >= s.MinScore {
			selectedAgents = append(selectedAgents, score.Agent)
		}
	}

	if len(selectedAgents) == 0 {
		// Fall back to default agent if available
		defaultAgent, err := s.AgentManager.GetAgent("default")
		if err != nil {
			return nil, fmt.Errorf("no suitable agent found and no default agent available")
		}
		return defaultAgent, nil
	}

	// Check if we can chain agents
	if len(selectedAgents) > 1 {
		chainedAgent, err := s.tryChainAgents(selectedAgents)
		if err == nil {
			return chainedAgent, nil
		}
		// If chaining fails, continue with the best matching agent
	}

	return selectedAgents[0], nil
}

func (s *SemanticAgentSelector) calculateSimilarity(taskDescription string, agent *Agent) float64 {
	// Calculate semantic similarity between task description and agent capabilities
	// This is a simplified version - in practice, you would use embeddings or a more
	// sophisticated similarity calculation

	// For now, we'll use a simple keyword matching as a placeholder
	score := 0.0
	description := agent.Description
	capabilities := agent.Capabilities

	// Check description match
    if containsAny(taskDescription, description) {
        score += 0.5
    }


	// Check capabilities match
	for _, capability := range capabilities {
		if containsAny(taskDescription, capability) {
			score += 0.3
		}
	}

	// Check required context availability
	state, err := s.AgentManager.GetAgentState(agent.Name)
	if err == nil && state != nil {
		contextScore := 0.0
		for _, required := range agent.RequiredContext {
			if _, exists := state.CurrentContext[required]; exists {
				contextScore += 0.2
			}
		}
		score += contextScore
	}

	return score
}

func (s *SemanticAgentSelector) tryChainAgents(agents []*Agent) (*Agent, error) {
	// This is a placeholder for agent chaining logic
	// In practice, you would implement more sophisticated chaining based on
	// agent compatibility and task requirements
	
	// For now, we'll just check if agents can chain based on the ChainsWith field
	for i := 0; i < len(agents)-1; i++ {
		canChain := false
		for _, chainable := range agents[i].ChainsWith {
			if chainable == agents[i+1].Name {
				canChain = true
				break
			}
		}
		if !canChain {
			return nil, fmt.Errorf("agents cannot be chained")
		}
	}

	// Create a new composite agent
	chainedAgent := &Agent{
		Name:        "chained-agent",
		Description: "Dynamically chained agent for complex tasks",
		PromptTemplates: map[string]string{
			"default": "You are a specialized agent combining multiple capabilities. Process the task using all available tools.",
		},
		Capabilities: []string{},
		MaxTokens:    4096, // Use a larger context for chained operations
		Temperature:  0.7,
	}

	// Combine capabilities
	for _, agent := range agents {
		chainedAgent.Capabilities = append(chainedAgent.Capabilities, agent.Capabilities...)
	}

	return chainedAgent, nil
}

// Helper function to check if any word from source is contained in target
func containsAny(source, target string) bool {
    if source == "" || target == "" {
        return false
    }
    source = strings.ToLower(source)
    target = strings.ToLower(target)

	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(source) + `\b`)
    return re.MatchString(target)
}