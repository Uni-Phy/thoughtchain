package tasks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// ExecutionMode represents how tasks should be executed
type ExecutionMode string

const (
	Sequential ExecutionMode = "sequential"
	Parallel   ExecutionMode = "parallel"
)

// TaskType represents the type of task
type TaskType string

const (
	FileSystemTask TaskType = "filesystem"
	APITask       TaskType = "api"
)

// Operation represents the operation to perform
type Operation string

const (
	// FileSystem operations
	ReadOperation   Operation = "read"
	WriteOperation  Operation = "write"
	DeleteOperation Operation = "delete"
	ListOperation   Operation = "list"

	// API operations
	GetOperation    Operation = "get"
	PostOperation   Operation = "post"
	PutOperation    Operation = "put"
	PatchOperation  Operation = "patch"
)

// Task represents a single executable task
type Task interface {
	Execute(ctx context.Context) error
	Description() string
	Dependencies() []string
}

// TaskDefinition defines the parameters for creating a task
type TaskDefinition struct {
	ID           string            `json:"id"`              // Unique identifier for the task
	Type         TaskType          `json:"type"`
	Operation    Operation         `json:"operation"`
	Path         string            `json:"path"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers"`
	Body         []byte            `json:"body"`
	ValidStatus  []int            `json:"valid_status"`
	DependsOn    []string         `json:"depends_on"`      // IDs of tasks this task depends on
	ExecutionMode ExecutionMode    `json:"execution_mode"` // How this task should be executed
}

// BaseTask contains common task fields
type BaseTask struct {
	id          string
	dependsOn   []string
}

func (b *BaseTask) Dependencies() []string {
	return b.dependsOn
}

// FileSystemTaskImpl implements file system operations
type FileSystemTaskImpl struct {
	BaseTask
	Operation Operation
	Path      string
	Content   []byte
}

func (f *FileSystemTaskImpl) Execute(ctx context.Context) error {
	switch f.Operation {
	case ReadOperation:
		_, err := os.Stat(f.Path)
		return err

	case WriteOperation:
		dir := filepath.Dir(f.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		return ioutil.WriteFile(f.Path, f.Content, 0644)

	case DeleteOperation:
		return os.Remove(f.Path)

	case ListOperation:
		files, err := ioutil.ReadDir(f.Path)
		if err != nil {
			return err
		}
		for _, file := range files {
			fmt.Println(file.Name())
		}
		return nil

	default:
		return fmt.Errorf("unsupported operation: %s", f.Operation)
	}
}

func (f *FileSystemTaskImpl) Description() string {
	return fmt.Sprintf("FileSystem operation: %s on path: %s", f.Operation, f.Path)
}

// APITaskImpl implements API operations
type APITaskImpl struct {
	BaseTask
	Method      string
	URL         string
	Headers     map[string]string
	Body        []byte
	ValidStatus []int
}

func (a *APITaskImpl) Execute(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, a.Method, a.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range a.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	validStatus := a.ValidStatus
	if len(validStatus) == 0 {
		validStatus = []int{http.StatusOK}
	}

	statusValid := false
	for _, status := range validStatus {
		if resp.StatusCode == status {
			statusValid = true
			break
		}
	}

	if !statusValid {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (a *APITaskImpl) Description() string {
	return fmt.Sprintf("API call: %s %s", a.Method, a.URL)
}

// TaskChain represents a sequence of tasks to be executed
type TaskChain struct {
	Tasks        map[string]Task // Map of task ID to task
	Dependencies map[string][]string // Map of task ID to dependent task IDs
	ExecutionMode ExecutionMode
}

// NewTaskChain creates a new task chain
func NewTaskChain() *TaskChain {
	return &TaskChain{
		Tasks:        make(map[string]Task),
		Dependencies: make(map[string][]string),
		ExecutionMode: Sequential,
	}
}

// AddTask adds a task to the chain
func (tc *TaskChain) AddTask(id string, task Task, dependsOn []string) {
	tc.Tasks[id] = task
	tc.Dependencies[id] = dependsOn
}

// Execute executes all tasks in the chain based on dependencies and execution mode
func (tc *TaskChain) Execute(ctx context.Context) error {
	if tc.ExecutionMode == Sequential {
		return tc.executeSequential(ctx)
	}
	return tc.executeParallel(ctx)
}

func (tc *TaskChain) executeSequential(ctx context.Context) error {
	executed := make(map[string]bool)
	
	var execute func(taskID string) error
	execute = func(taskID string) error {
		if executed[taskID] {
			return nil
		}

		// Execute dependencies first
		for _, depID := range tc.Dependencies[taskID] {
			if err := execute(depID); err != nil {
				return err
			}
		}

		task := tc.Tasks[taskID]
		if err := task.Execute(ctx); err != nil {
			return fmt.Errorf("task failed: %s: %w", task.Description(), err)
		}

		executed[taskID] = true
		return nil
	}

	// Execute all tasks
	for id := range tc.Tasks {
		if err := execute(id); err != nil {
			return err
		}
	}

	return nil
}

func (tc *TaskChain) executeParallel(ctx context.Context) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(tc.Tasks))
	executed := make(map[string]bool)
	var executedMu sync.Mutex

	var execute func(taskID string)
	execute = func(taskID string) {
		defer wg.Done()

		executedMu.Lock()
		if executed[taskID] {
			executedMu.Unlock()
			return
		}
		executedMu.Unlock()

		// Wait for dependencies
		for _, depID := range tc.Dependencies[taskID] {
			wg.Add(1)
			go execute(depID)
		}

		task := tc.Tasks[taskID]
		if err := task.Execute(ctx); err != nil {
			errChan <- fmt.Errorf("task failed: %s: %w", task.Description(), err)
			return
		}

		executedMu.Lock()
		executed[taskID] = true
		executedMu.Unlock()
	}

	// Start execution of all tasks
	for id := range tc.Tasks {
		wg.Add(1)
		go execute(id)
	}

	// Wait for all tasks to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// CreateTask creates a task from a task definition
func CreateTask(def TaskDefinition) (Task, error) {
	var baseTask = BaseTask{
		id:        def.ID,
		dependsOn: def.DependsOn,
	}

	switch def.Type {
	case FileSystemTask:
		return &FileSystemTaskImpl{
			BaseTask:  baseTask,
			Operation: def.Operation,
			Path:      def.Path,
			Content:   def.Body,
		}, nil
	case APITask:
		return &APITaskImpl{
			BaseTask:    baseTask,
			Method:      def.Method,
			URL:         def.URL,
			Headers:     def.Headers,
			Body:        def.Body,
			ValidStatus: def.ValidStatus,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported task type: %s", def.Type)
	}
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}
