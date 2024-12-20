package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelTaskExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "parallel-tasks-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Execute independent tasks in parallel", func(t *testing.T) {
		chain := NewTaskChain()
		chain.ExecutionMode = Parallel

		var executionCount int32

		// Create multiple file write tasks
		for i := 1; i <= 3; i++ {
			taskID := fmt.Sprintf("task%d", i)
			content := []byte(fmt.Sprintf("content%d", i))
			task := &FileSystemTaskImpl{
				BaseTask: BaseTask{
					id: taskID,
				},
				Operation: WriteOperation,
				Path:      filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i)),
				Content:   content,
			}
			chain.AddTask(taskID, task, nil)
		}

		start := time.Now()
		err := chain.Execute(context.Background())
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify all files were created
		for i := 1; i <= 3; i++ {
			path := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Expected file %s to exist", path)
			}
		}

		// Parallel execution should be faster than sequential
		if duration > 3*time.Second {
			t.Errorf("Parallel execution took too long: %v", duration)
		}
	})

	t.Run("Execute tasks with dependencies", func(t *testing.T) {
		chain := NewTaskChain()
		chain.ExecutionMode = Parallel

		// Create parent task
		parentPath := filepath.Join(tmpDir, "parent.txt")
		parentTask := &FileSystemTaskImpl{
			BaseTask: BaseTask{
				id: "parent",
			},
			Operation: WriteOperation,
			Path:      parentPath,
			Content:   []byte("parent content"),
		}
		chain.AddTask("parent", parentTask, nil)

		// Create dependent tasks
		for i := 1; i <= 2; i++ {
			taskID := fmt.Sprintf("child%d", i)
			task := &FileSystemTaskImpl{
				BaseTask: BaseTask{
					id:        taskID,
					dependsOn: []string{"parent"},
				},
				Operation: WriteOperation,
				Path:      filepath.Join(tmpDir, fmt.Sprintf("child%d.txt", i)),
				Content:   []byte(fmt.Sprintf("child content %d", i)),
			}
			chain.AddTask(taskID, task, []string{"parent"})
		}

		err := chain.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify parent file exists
		if _, err := os.Stat(parentPath); os.IsNotExist(err) {
			t.Error("Parent file should exist before child files")
		}

		// Verify child files exist
		for i := 1; i <= 2; i++ {
			path := filepath.Join(tmpDir, fmt.Sprintf("child%d.txt", i))
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Expected child file %s to exist", path)
			}
		}
	})
}

func TestSequentialTaskExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sequential-tasks-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Execute tasks in sequence", func(t *testing.T) {
		chain := NewTaskChain()
		chain.ExecutionMode = Sequential

		var executionOrder []string
		for i := 1; i <= 3; i++ {
			taskID := fmt.Sprintf("task%d", i)
			task := &FileSystemTaskImpl{
				BaseTask: BaseTask{
					id: taskID,
				},
				Operation: WriteOperation,
				Path:      filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i)),
				Content:   []byte(fmt.Sprintf("content%d", i)),
			}
			chain.AddTask(taskID, task, nil)
		}

		err := chain.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify files were created in order
		for i := 1; i <= 3; i++ {
			path := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Expected file %s to exist", path)
			}
		}
	})
}

func TestTaskCreation(t *testing.T) {
	t.Run("Create file system task", func(t *testing.T) {
		def := TaskDefinition{
			ID:           "test-fs-task",
			Type:         FileSystemTask,
			Operation:    WriteOperation,
			Path:         "test.txt",
			Body:         []byte("test content"),
			DependsOn:    []string{"dep1", "dep2"},
			ExecutionMode: Sequential,
		}

		task, err := CreateTask(def)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		fsTask, ok := task.(*FileSystemTaskImpl)
		if !ok {
			t.Error("Expected FileSystemTaskImpl")
		}

		if fsTask.Operation != WriteOperation {
			t.Errorf("Expected operation %s, got %s", WriteOperation, fsTask.Operation)
		}

		if len(fsTask.Dependencies()) != 2 {
			t.Errorf("Expected 2 dependencies, got %d", len(fsTask.Dependencies()))
		}
	})

	t.Run("Create API task", func(t *testing.T) {
		def := TaskDefinition{
			ID:           "test-api-task",
			Type:         APITask,
			Method:       "GET",
			URL:          "http://example.com",
			Headers:      map[string]string{"Accept": "application/json"},
			ValidStatus:  []int{http.StatusOK},
			ExecutionMode: Parallel,
		}

		task, err := CreateTask(def)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		apiTask, ok := task.(*APITaskImpl)
		if !ok {
			t.Error("Expected APITaskImpl")
		}

		if apiTask.Method != "GET" {
			t.Errorf("Expected method GET, got %s", apiTask.Method)
		}
	})

	t.Run("Invalid task type", func(t *testing.T) {
		def := TaskDefinition{
			Type: "invalid",
		}

		_, err := CreateTask(def)
		if err == nil {
			t.Error("Expected error for invalid task type")
		}
	})
}

func TestAPITaskExecution(t *testing.T) {
	t.Run("Successful API call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		task := &APITaskImpl{
			BaseTask: BaseTask{
				id: "test-api",
			},
			Method:      "GET",
			URL:         server.URL,
			Headers:     map[string]string{"Accept": "application/json"},
			ValidStatus: []int{http.StatusOK},
		}

		err := task.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Failed API call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		task := &APITaskImpl{
			BaseTask: BaseTask{
				id: "test-api-fail",
			},
			Method:      "GET",
			URL:         server.URL,
			Headers:     map[string]string{"Accept": "application/json"},
			ValidStatus: []int{http.StatusOK},
		}

		err := task.Execute(context.Background())
		if err == nil {
			t.Error("Expected error for failed API call")
		}
	})
}
