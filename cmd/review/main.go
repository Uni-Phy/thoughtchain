package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"thoughtchain/llms"
)

const defaultReviewPrompt = `You are a senior software engineer performing a code review. Analyze the following code structure and provide detailed feedback:

Code Structure:
%s

Please provide feedback on:
1. Code organization and package structure
2. Function and type design
3. Documentation completeness
4. Potential improvements
5. Best practices adherence
6. Any security concerns
7. Performance considerations
8. Testing coverage

Focus on actionable suggestions and specific improvements for each file.`

const defaultFilePrompt = `Review the following Go file and provide specific suggestions for improvement:

File: %s
Package: %s
Contents:

%s

Please provide specific suggestions for:
1. Code organization
2. Error handling
3. Documentation
4. Testing
5. Performance
6. Security
7. Best practices

Focus on actionable, concrete improvements.`

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if entry.Name() == ".thoughtchain" {
				continue
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func loadPromptFromFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file: %w", err)
	}
	return string(content), nil
}

func main() {
	// Parse command line flags
	dir := flag.String("dir", ".", "Directory to analyze")
	apiKey := flag.String("api-key", os.Getenv("OPENROUTER_API_KEY"), "OpenRouter API key")
	reviewPromptFile := flag.String("review-prompt", "", "Path to custom review prompt template file")
	filePromptFile := flag.String("file-prompt", "", "Path to custom file review prompt template file")
	task := flag.String("task", "", "Specific task for code modification (e.g., 'add error handling', 'implement logging')")
	mode := flag.String("mode", "review", "Operation mode: 'review' (default) or 'modify'")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("OpenRouter API key is required. Set OPENROUTER_API_KEY environment variable or use -api-key flag")
	}

	// Load custom prompts if provided
	reviewPrompt := defaultReviewPrompt
	filePrompt := defaultFilePrompt

	if *reviewPromptFile != "" {
		if content, err := loadPromptFromFile(*reviewPromptFile); err != nil {
			log.Fatalf("Failed to load review prompt: %v", err)
		} else {
			reviewPrompt = content
		}
	}

	if *filePromptFile != "" {
		if content, err := loadPromptFromFile(*filePromptFile); err != nil {
			log.Fatalf("Failed to load file prompt: %v", err)
		} else {
			filePrompt = content
		}
	}

	// Modify prompts based on task if provided
	if *task != "" {
		if *mode == "modify" {
			reviewPrompt = fmt.Sprintf(`You are a senior software engineer tasked with: %s

Analyze the following code structure and provide specific modifications needed:

Code Structure:
%%s

Focus on:
1. Required code changes
2. New functions or methods needed
3. Modifications to existing code
4. Any new files needed
5. Testing requirements

Provide detailed, implementation-ready suggestions.`, *task)

			filePrompt = fmt.Sprintf(`You are a senior software engineer tasked with: %s

Analyze the following file and provide specific code changes:

File: %%s
Package: %%s
Contents:

%%s

Provide:
1. Exact code modifications needed
2. New code to be added
3. Required refactoring
4. Updated tests

Write complete, implementation-ready code.`, *task)
		} else {
			reviewPrompt = fmt.Sprintf(`You are a senior software engineer reviewing code with focus on: %s

Analyze the following code structure and provide targeted feedback:

Code Structure:
%%s

Focus your review on aspects relevant to: %s`, *task, *task)

			filePrompt = fmt.Sprintf(`You are a senior software engineer reviewing code with focus on: %s

Review the following file:

File: %%s
Package: %%s
Contents:

%%s

Provide specific feedback and suggestions related to: %s`, *task, *task)
		}
	}

	// Create .thoughtchain directory
	thoughtchainDir := filepath.Join(*dir, ".thoughtchain")
	if err := os.MkdirAll(thoughtchainDir, 0755); err != nil {
		log.Fatalf("Failed to create .thoughtchain directory: %v", err)
	}

	// Create subdirectories
	reviewDir := filepath.Join(thoughtchainDir, "review")
	workingDir := filepath.Join(thoughtchainDir, "working")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		log.Fatalf("Failed to create review directory: %v", err)
	}
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		log.Fatalf("Failed to create working directory: %v", err)
	}

	// Clone the codebase to working directory
	log.Printf("Cloning codebase to working directory...")
	if err := copyDir(*dir, workingDir); err != nil {
		log.Fatalf("Failed to clone codebase: %v", err)
	}

	// Initialize code analyzer
	analyzer := NewAnalyzer()

	// Analyze code
	log.Printf("Analyzing directory: %s", *dir)
	if err := analyzer.AnalyzeDirectory(*dir); err != nil {
		log.Fatalf("Failed to analyze directory: %v", err)
	}

	// Generate documentation
	doc := analyzer.GenerateDocumentation()
	docPath := filepath.Join(reviewDir, "structure.md")
	if err := os.WriteFile(docPath, []byte(doc), 0644); err != nil {
		log.Fatalf("Failed to write documentation: %v", err)
	}
	log.Printf("Generated code structure documentation: %s", docPath)

	// Save code structure
	structure := analyzer.GetStructure()
	structurePath := filepath.Join(reviewDir, "structure.json")
	structureJSON, err := json.MarshalIndent(structure, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal structure: %v", err)
	}
	if err := os.WriteFile(structurePath, structureJSON, 0644); err != nil {
		log.Fatalf("Failed to write structure: %v", err)
	}
	log.Printf("Saved code structure: %s", structurePath)

	// Initialize LLM client
	config := llms.OpenRouterConfig{
		APIKey:  *apiKey,
		BaseURL: "https://openrouter.ai/api/v1/chat/completions",
	}
	llmClient := llms.NewOpenRouterClient(config, log.Default())

	// Generate review using LLM
	log.Println("Generating code review...")
	prompt := fmt.Sprintf(reviewPrompt, doc)
	thoughts, err := llmClient.GenerateThoughts(context.Background(), prompt)
	if err != nil {
		log.Fatalf("Failed to generate review: %v", err)
	}

	// Save review
	review := strings.Join(thoughts, "\n\n")
	reviewPath := filepath.Join(reviewDir, "review.md")
	if err := os.WriteFile(reviewPath, []byte(review), 0644); err != nil {
		log.Fatalf("Failed to write review: %v", err)
	}
	log.Printf("Generated code review: %s", reviewPath)

	// Generate file-specific suggestions
	log.Println("Generating file-specific suggestions...")
	for path, file := range structure.Files {
		relPath := strings.TrimPrefix(path, *dir)
		relPath = strings.TrimPrefix(relPath, "/")
		reviewFilePath := filepath.Join(reviewDir, relPath+".review.md")

		if err := os.MkdirAll(filepath.Dir(reviewFilePath), 0755); err != nil {
			log.Printf("Failed to create directory for %s: %v", reviewFilePath, err)
			continue
		}

		filePromptContent := fmt.Sprintf(filePrompt, path, file.Package, file.Contents)
		thoughts, err := llmClient.GenerateThoughts(context.Background(), filePromptContent)
		if err != nil {
			log.Printf("Failed to generate suggestions for %s: %v", path, err)
			continue
		}

		suggestions := strings.Join(thoughts, "\n\n")
		if err := os.WriteFile(reviewFilePath, []byte(suggestions), 0644); err != nil {
			log.Printf("Failed to write suggestions for %s: %v", path, err)
			continue
		}
		log.Printf("Generated suggestions for %s: %s", path, reviewFilePath)

		// In modify mode, also write modified code to working directory
		if *mode == "modify" {
			modifyPrompt := fmt.Sprintf(`Given the code review suggestions, generate the complete modified version of the file:

Original File:
%s

Review Suggestions:
%s

Provide the complete, modified file content incorporating all suggested changes.`, file.Contents, suggestions)

			thoughts, err := llmClient.GenerateThoughts(context.Background(), modifyPrompt)
			if err != nil {
				log.Printf("Failed to generate modified code for %s: %v", path, err)
				continue
			}

			modifiedCode := strings.Join(thoughts, "\n\n")
			modifiedPath := filepath.Join(workingDir, relPath)
			if err := os.WriteFile(modifiedPath, []byte(modifiedCode), 0644); err != nil {
				log.Printf("Failed to write modified code for %s: %v", path, err)
				continue
			}
			log.Printf("Generated modified code for %s: %s", path, modifiedPath)
		}
	}

	log.Printf("\nCode review completed successfully!")
	log.Printf("\nArtifacts are stored in: %s", thoughtchainDir)
	log.Printf("- Review files: %s", reviewDir)
	log.Printf("- Working copy: %s", workingDir)
}
