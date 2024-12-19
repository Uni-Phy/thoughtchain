package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// LLMClient defines the interface for LLM interactions
type LLMClient interface {
	GenerateThoughts(ctx context.Context, query string) ([]string, error)
	ConcludeThoughts(ctx context.Context, thoughts []string) (string, error)
}


type OpenRouterConfig struct {
	APIKey string
	BaseURL string
}

// OpenRouterLLMClient implements LLMClient using OpenRouter API
type OpenRouterLLMClient struct {
	Config OpenRouterConfig
	HTTPClient *http.Client
    Logger *log.Logger
}


// OpenRouterResponse represents the API response structure
type OpenRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}


// NewOpenRouterClient creates a new OpenRouter client
func NewOpenRouterClient(config OpenRouterConfig, logger *log.Logger) *OpenRouterLLMClient {
	return &OpenRouterLLMClient{
		Config:     config,
		HTTPClient: &http.Client{},
        Logger: logger,
	}
}

// makeRequest handles common HTTP request functionality
func (o *OpenRouterLLMClient) makeRequest(ctx context.Context, messages []map[string]string) (*OpenRouterResponse, error) {
    o.Logger.Printf("Making request to OpenRouter API: %v", messages)
	requestBody, err := json.Marshal(map[string]interface{}{
		"messages": messages,
		"model":    "meta-llama/llama-3.2-3b-instruct:free",
	})
	if err != nil {
		o.Logger.Printf("Failed to encode request body: %v", err)
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.Config.BaseURL, bytes.NewBuffer(requestBody))
	if err != nil {
        o.Logger.Printf("Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.Config.APIKey))

	resp, err := o.HTTPClient.Do(req)
	if err != nil {
        o.Logger.Printf("Failed to execute request: %v", err)
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		o.Logger.Printf("Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	o.Logger.Printf("OpenRouter API response status code: %d and response body: %s", resp.StatusCode, string(body))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api failed with status code: %d and response: %s", resp.StatusCode, string(body))
	}

	var openRouterResponse OpenRouterResponse
	if err := json.Unmarshal(body, &openRouterResponse); err != nil {
		o.Logger.Printf("Failed to unmarshal response body %s: %v", string(body), err)
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &openRouterResponse, nil
}

func (o *OpenRouterLLMClient) GenerateThoughts(ctx context.Context, query string) ([]string, error) {
    o.Logger.Printf("Generating thoughts with query: %s", query)
	if query == "" {
        o.Logger.Println("Empty query")
		return nil, errors.New("empty query")
	}

	response, err := o.makeRequest(ctx, []map[string]string{
		{
			"role":    "user",
			"content": query,
		},
	})
	if err != nil {
		o.Logger.Printf("Failed to generate thoughts: %v", err)
		return nil, err
	}
    
	var thoughts []string
	for _, choice := range response.Choices {
		thoughts = append(thoughts, choice.Message.Content)
	}

	o.Logger.Printf("Generated thoughts: %v", thoughts)

	return thoughts, nil
}


func (o *OpenRouterLLMClient) ConcludeThoughts(ctx context.Context, thoughts []string) (string, error) {
    o.Logger.Printf("Generating conclusion with thoughts: %v", thoughts)
	if len(thoughts) == 0 {
        o.Logger.Println("No thoughts to conclude")
		return "", errors.New("no thoughts provided")
	}

	response, err := o.makeRequest(ctx, []map[string]string{
		{
			"role":    "user",
			"content": fmt.Sprintf("Summarize the following thoughts into a conclusion: %s", thoughts),
		},
	})
	if err != nil {
		o.Logger.Printf("Failed to generate conclusion: %v", err)
		return "", err
	}


	if len(response.Choices) > 0 {
        o.Logger.Printf("Generated conclusion: %s", response.Choices[0].Message.Content)
		return response.Choices[0].Message.Content, nil
	}

	o.Logger.Println("No conclusion generated")
	return "", errors.New("no conclusion generated")
}