package tasks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSystemTask(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tasks-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Write operation", func(t *testing.T) {
		content := []byte("test content")
		task := &FileSystemTaskImpl{
			Operation: WriteOperation,
			Path:      filepath.Join(tmpDir, "test.txt"),
			Content:   content,
		}

		err := task.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify file was created
		data, err := os.ReadFile(task.Path)
		if err != nil {
			t.Errorf("Failed to read file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("Expected content %q, got %q", content, data)
		}
	})

	t.Run("Read operation", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "read-test.txt")
		content := []byte("read test content")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatal(err)
		}

		task := &FileSystemTaskImpl{
			Operation: ReadOperation,
			Path:      filePath,
		}

		err := task.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Delete operation", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "delete-test.txt")
		if err := os.WriteFile(filePath, []byte("delete me"), 0644); err != nil {
			t.Fatal(err)
		}

		task := &FileSystemTaskImpl{
			Operation: DeleteOperation,
			Path:      filePath,
		}

		err := task.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify file was deleted
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Error("Expected file to be deleted")
		}
	})
}

func TestAPITask(t *testing.T) {
	t.Run("Successful API call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		task := &APITaskImpl{
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

	t.Run("Invalid status code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		task := &APITaskImpl{
			Method:      "GET",
			URL:         server.URL,
			Headers:     map[string]string{"Accept": "application/json"},
			ValidStatus: []int{http.StatusOK},
		}

		err := task.Execute(context.Background())
		if err == nil {
			t.Error("Expected error for invalid status code")
		}
	})
}

func TestTaskChain(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskchain-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Execute multiple tasks", func(t *testing.T) {
		chain := NewTaskChain()

		// Add write task
		writeTask := &FileSystemTaskImpl{
			Operation: WriteOperation,
			Path:      filepath.Join(tmpDir, "chain-test.txt"),
			Content:   []byte("chain test"),
		}
		chain.AddTask(writeTask)

		// Add read task
		readTask := &FileSystemTaskImpl{
			Operation: ReadOperation,
			Path:      writeTask.Path,
		}
		chain.AddTask(readTask)

		err := chain.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Task failure handling", func(t *testing.T) {
		chain := NewTaskChain()

		// Add task that will fail (reading non-existent file)
		failTask := &FileSystemTaskImpl{
			Operation: ReadOperation,
			Path:      filepath.Join(tmpDir, "nonexistent.txt"),
		}
		chain.AddTask(failTask)

		err := chain.Execute(context.Background())
		if err == nil {
			t.Error("Expected error for failed task")
		}
	})
}

func TestCreateTask(t *testing.T) {
	t.Run("Create filesystem task", func(t *testing.T) {
		def := TaskDefinition{
			Type:      FileSystemTask,
			Operation: WriteOperation,
			Path:      "test.txt",
			Body:      []byte("test"),
		}

		task, err := CreateTask(def)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if task == nil {
			t.Error("Expected task to be created")
		}
	})

	t.Run("Create API task", func(t *testing.T) {
		def := TaskDefinition{
			Type:   APITask,
			Method: "GET",
			URL:    "http://example.com",
			Headers: map[string]string{
				"Accept": "application/json",
			},
		}

		task, err := CreateTask(def)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if task == nil {
			t.Error("Expected task to be created")
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
