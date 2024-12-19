package tasks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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
	WriteOperation Operation = "write"
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
}

// TaskDefinition defines the parameters for creating a task
type TaskDefinition struct {
	Type        TaskType           `json:"type"`
	Operation   Operation          `json:"operation"`
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
	ValidStatus []int             `json:"valid_status"`
}

// FileSystemTaskImpl implements file system operations
type FileSystemTaskImpl struct {
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

	// Validate response status
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
	Tasks []Task
}

// NewTaskChain creates a new task chain
func NewTaskChain() *TaskChain {
	return &TaskChain{
		Tasks: make([]Task, 0),
	}
}

// AddTask adds a task to the chain
func (tc *TaskChain) AddTask(task Task) {
	tc.Tasks = append(tc.Tasks, task)
}

// Execute executes all tasks in the chain
func (tc *TaskChain) Execute(ctx context.Context) error {
	for _, task := range tc.Tasks {
		if err := task.Execute(ctx); err != nil {
			return fmt.Errorf("task failed: %s: %w", task.Description(), err)
		}
	}
	return nil
}

// CreateTask creates a task from a task definition
func CreateTask(def TaskDefinition) (Task, error) {
	switch def.Type {
	case FileSystemTask:
		return &FileSystemTaskImpl{
			Operation: def.Operation,
			Path:      def.Path,
			Content:   def.Body,
		}, nil
	case APITask:
		return &APITaskImpl{
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
	Description string `json:"description"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}
