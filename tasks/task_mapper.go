package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// TaskPattern represents a pattern for matching prompts to task templates
type TaskPattern struct {
	Pattern     string            `json:"pattern"`     // Regex pattern to match prompt
	Template    TaskDefinition    `json:"template"`    // Task template to use
	Variables   []string          `json:"variables"`   // Variables to extract from prompt
	DependsOn   []string          `json:"depends_on"`  // Task IDs this task depends on
	IsParallel  bool             `json:"is_parallel"` // Whether this task can run in parallel
}

// TaskMapper handles mapping prompts to task definitions
type TaskMapper struct {
	patterns []*TaskPattern
}

// NewTaskMapper creates a new task mapper
func NewTaskMapper() *TaskMapper {
	return &TaskMapper{
		patterns: make([]*TaskPattern, 0),
	}
}

// RegisterPattern registers a new task pattern
func (tm *TaskMapper) RegisterPattern(pattern *TaskPattern) error {
	// Validate pattern
	if _, err := regexp.Compile(pattern.Pattern); err != nil {
		return fmt.Errorf("invalid pattern regex: %w", err)
	}

	// Validate template
	if pattern.Template.Type == "" {
		return fmt.Errorf("task template must specify a type")
	}

	tm.patterns = append(tm.patterns, pattern)
	return nil
}

// MapPromptToTasks maps a prompt to a set of task definitions
func (tm *TaskMapper) MapPromptToTasks(ctx context.Context, prompt string) (*TaskChain, error) {
	chain := NewTaskChain()
	
	// Find all matching patterns
	var matchedTasks []struct {
		pattern *TaskPattern
		matches []string
		vars    map[string]string
	}

	for _, pattern := range tm.patterns {
		re, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			continue
		}

		matches := re.FindStringSubmatch(prompt)
		if len(matches) > 0 {
			vars := make(map[string]string)
			for i, name := range pattern.Variables {
				if i+1 < len(matches) {
					vars[name] = matches[i+1]
				}
			}
			matchedTasks = append(matchedTasks, struct {
				pattern *TaskPattern
				matches []string
				vars    map[string]string
			}{pattern, matches, vars})
		}
	}

	// Group tasks by execution mode
	var sequentialTasks, parallelTasks []struct {
		taskDef TaskDefinition
		deps    []string
	}

	for i, matched := range matchedTasks {
		// Clone the template
		taskDef := matched.pattern.Template
		taskDef.ID = fmt.Sprintf("task-%d", i)

		// Replace variables in task definition
		taskDefJSON, _ := json.Marshal(taskDef)
		taskDefStr := string(taskDefJSON)
		for name, value := range matched.vars {
			taskDefStr = strings.ReplaceAll(taskDefStr, fmt.Sprintf("${%s}", name), value)
		}
		json.Unmarshal([]byte(taskDefStr), &taskDef)

		// Add dependencies
		deps := make([]string, len(matched.pattern.DependsOn))
		copy(deps, matched.pattern.DependsOn)

		if matched.pattern.IsParallel {
			parallelTasks = append(parallelTasks, struct {
				taskDef TaskDefinition
				deps    []string
			}{taskDef, deps})
		} else {
			sequentialTasks = append(sequentialTasks, struct {
				taskDef TaskDefinition
				deps    []string
			}{taskDef, deps})
		}
	}

	// Add sequential tasks first
	for _, t := range sequentialTasks {
		task, err := CreateTask(t.taskDef)
		if err != nil {
			return nil, fmt.Errorf("failed to create sequential task: %w", err)
		}
		chain.AddTask(t.taskDef.ID, task, t.deps)
	}

	// Then add parallel tasks
	if len(parallelTasks) > 0 {
		chain.ExecutionMode = Parallel
		for _, t := range parallelTasks {
			task, err := CreateTask(t.taskDef)
			if err != nil {
				return nil, fmt.Errorf("failed to create parallel task: %w", err)
			}
			chain.AddTask(t.taskDef.ID, task, t.deps)
		}
	}

	return chain, nil
}

// TaskGroup represents a group of related tasks
type TaskGroup struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	Patterns     []*TaskPattern   `json:"patterns"`
	ExecutionMode ExecutionMode   `json:"execution_mode"`
}

// RegisterTaskGroup registers a group of related task patterns
func (tm *TaskMapper) RegisterTaskGroup(group *TaskGroup) error {
	for _, pattern := range group.Patterns {
		pattern.IsParallel = group.ExecutionMode == Parallel
		if err := tm.RegisterPattern(pattern); err != nil {
			return fmt.Errorf("failed to register pattern in group %s: %w", group.Name, err)
		}
	}
	return nil
}
