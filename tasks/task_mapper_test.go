package tasks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTaskMapper(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "task-mapper-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("Register and match simple pattern", func(t *testing.T) {
		mapper := NewTaskMapper()

		pattern := &TaskPattern{
			Pattern: `write file (\w+\.txt) with content (.+)`,
			Variables: []string{"filename", "content"},
			Template: TaskDefinition{
				Type:      FileSystemTask,
				Operation: WriteOperation,
				Path:      "${filename}",
				Body:      []byte("${content}"),
			},
		}

		err := mapper.RegisterPattern(pattern)
		if err != nil {
			t.Errorf("Failed to register pattern: %v", err)
		}

		chain, err := mapper.MapPromptToTasks(context.Background(), "write file test.txt with content hello world")
		if err != nil {
			t.Errorf("Failed to map prompt to tasks: %v", err)
		}

		if len(chain.Tasks) != 1 {
			t.Errorf("Expected 1 task, got %d", len(chain.Tasks))
		}

		// Execute the task chain
		for id, task := range chain.Tasks {
			fsTask, ok := task.(*FileSystemTaskImpl)
			if !ok {
				t.Errorf("Task %s is not a FileSystemTaskImpl", id)
				continue
			}

			if fsTask.Operation != WriteOperation {
				t.Errorf("Expected WriteOperation, got %s", fsTask.Operation)
			}

			if fsTask.Path != "test.txt" {
				t.Errorf("Expected path test.txt, got %s", fsTask.Path)
			}

			if string(fsTask.Content) != "hello world" {
				t.Errorf("Expected content 'hello world', got '%s'", string(fsTask.Content))
			}
		}
	})

	t.Run("Match parallel tasks", func(t *testing.T) {
		mapper := NewTaskMapper()

		group := &TaskGroup{
			Name:         "file-ops",
			Description: "File operations that can run in parallel",
			ExecutionMode: Parallel,
			Patterns: []*TaskPattern{
				{
					Pattern: `create directory (\w+)`,
					Variables: []string{"dirname"},
					Template: TaskDefinition{
						Type:      FileSystemTask,
						Operation: WriteOperation,
						Path:      "${dirname}",
					},
				},
				{
					Pattern: `write file (\w+\.txt) in (\w+) with content (.+)`,
					Variables: []string{"filename", "dirname", "content"},
					Template: TaskDefinition{
						Type:      FileSystemTask,
						Operation: WriteOperation,
						Path:      "${dirname}/${filename}",
						Body:      []byte("${content}"),
					},
				},
			},
		}

		err := mapper.RegisterTaskGroup(group)
		if err != nil {
			t.Errorf("Failed to register task group: %v", err)
		}

		prompt := `
			create directory testdir
			write file file1.txt in testdir with content hello
			write file file2.txt in testdir with content world
		`

		chain, err := mapper.MapPromptToTasks(context.Background(), prompt)
		if err != nil {
			t.Errorf("Failed to map prompt to tasks: %v", err)
		}

		if chain.ExecutionMode != Parallel {
			t.Error("Expected parallel execution mode")
		}

		if len(chain.Tasks) != 3 {
			t.Errorf("Expected 3 tasks, got %d", len(chain.Tasks))
		}
	})

	t.Run("Match tasks with dependencies", func(t *testing.T) {
		mapper := NewTaskMapper()

		// Register patterns for a file creation workflow
		createDirPattern := &TaskPattern{
			Pattern: `create directory (\w+)`,
			Variables: []string{"dirname"},
			Template: TaskDefinition{
				ID:        "create-dir",
				Type:      FileSystemTask,
				Operation: WriteOperation,
				Path:      "${dirname}",
			},
		}

		writeFilePattern := &TaskPattern{
			Pattern: `write file (\w+\.txt) in (\w+) with content (.+)`,
			Variables: []string{"filename", "dirname", "content"},
			Template: TaskDefinition{
				Type:      FileSystemTask,
				Operation: WriteOperation,
				Path:      "${dirname}/${filename}",
				Body:      []byte("${content}"),
			},
			DependsOn: []string{"create-dir"},
		}

		err := mapper.RegisterPattern(createDirPattern)
		if err != nil {
			t.Errorf("Failed to register create dir pattern: %v", err)
		}

		err = mapper.RegisterPattern(writeFilePattern)
		if err != nil {
			t.Errorf("Failed to register write file pattern: %v", err)
		}

		prompt := `
			create directory testdir
			write file test.txt in testdir with content hello world
		`

		chain, err := mapper.MapPromptToTasks(context.Background(), prompt)
		if err != nil {
			t.Errorf("Failed to map prompt to tasks: %v", err)
		}

		// Execute the chain
		err = chain.Execute(context.Background())
		if err != nil {
			t.Errorf("Failed to execute task chain: %v", err)
		}

		// Verify the directory and file were created in the correct order
		dirPath := filepath.Join(tmpDir, "testdir")
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Error("Directory was not created")
		}

		filePath := filepath.Join(dirPath, "test.txt")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("File was not created")
		}
	})

	t.Run("Invalid pattern registration", func(t *testing.T) {
		mapper := NewTaskMapper()

		pattern := &TaskPattern{
			Pattern: `[invalid regex`,
			Template: TaskDefinition{
				Type: FileSystemTask,
			},
		}

		err := mapper.RegisterPattern(pattern)
		if err == nil {
			t.Error("Expected error for invalid regex pattern")
		}

		pattern = &TaskPattern{
			Pattern: `valid pattern`,
			Template: TaskDefinition{}, // Missing type
		}

		err = mapper.RegisterPattern(pattern)
		if err == nil {
			t.Error("Expected error for invalid task template")
		}
	})

	t.Run("No matching patterns", func(t *testing.T) {
		mapper := NewTaskMapper()

		chain, err := mapper.MapPromptToTasks(context.Background(), "this won't match any patterns")
		if err != nil {
			t.Errorf("Expected no error for non-matching prompt, got %v", err)
		}

		if len(chain.Tasks) != 0 {
			t.Errorf("Expected 0 tasks for non-matching prompt, got %d", len(chain.Tasks))
		}
	})
}
