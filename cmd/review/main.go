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
	"thoughtchain/review"
)

const reviewPromptTemplate = `You are a senior software engineer performing a code review. Analyze the following code structure and provide detailed feedback:

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

func copyDir(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dst, err)
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Skip .thoughtchain directory to avoid recursive copying
			if entry.Name() == ".thoughtchain" {
				continue
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
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

func main() {
	// Parse command line flags
	dir := flag.String("dir", ".", "Directory to analyze")
	apiKey := flag.String("api-key", os.Getenv("OPENROUTER_API_KEY"), "OpenRouter API key")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("OpenRouter API key is required. Set OPENROUTER_API_KEY environment variable or use -api-key flag")
	}

	// Create .thoughtchain directory in the target directory
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
	analyzer := review.NewAnalyzer()

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

	// Save code structure as JSON
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
	prompt := fmt.Sprintf(reviewPromptTemplate, doc)
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
		// Create relative path structure in review directory
		relPath := strings.TrimPrefix(path, *dir)
		relPath = strings.TrimPrefix(relPath, "/")
		reviewFilePath := filepath.Join(reviewDir, relPath+".review.md")

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(reviewFilePath), 0755); err != nil {
			log.Printf("Failed to create directory for %s: %v", reviewFilePath, err)
			continue
		}

		filePrompt := fmt.Sprintf(`Review the following Go file and provide specific suggestions for improvement:

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

Focus on actionable, concrete improvements.`, path, file.Package, file.Contents)

		thoughts, err := llmClient.GenerateThoughts(context.Background(), filePrompt)
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
	}

	log.Printf("\nCode review completed successfully!")
	log.Printf("\nReview artifacts are stored in: %s", thoughtchainDir)
	log.Printf("- Review files: %s", reviewDir)
	log.Printf("- Working copy: %s", workingDir)
}
