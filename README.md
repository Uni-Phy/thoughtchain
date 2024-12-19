# ThoughtChain

ThoughtChain is a sophisticated Go library for executing sequences of tasks using specialized agents powered by LLM-based thought generation. It provides a flexible and extensible system for handling various types of operations including filesystem tasks, API interactions, text processing, and code review.

## Prerequisites

1. Go 1.16 or higher
2. OpenRouter API Key (Get one from [OpenRouter](https://openrouter.ai/))

## Installation

```bash
go get github.com/Uni-Phy/thoughtchain/
```

## Environment Setup

1. Set your OpenRouter API key:
```bash
export OPENROUTER_API_KEY=your_api_key_here
```

2. Create the agent configuration directory:
```bash
mkdir -p config/agents
```

## Code Review Tool

ThoughtChain includes a powerful code review tool that can analyze Go code and provide detailed feedback using LLM-based analysis.

### Running Code Reviews

```bash
# Review current directory
go run cmd/review/main.go

# Review specific directory
go run cmd/review/main.go -dir=/path/to/project

# Specify output directory
go run cmd/review/main.go -dir=/path/to/project -output=review-output

# Use specific API key
go run cmd/review/main.go -api-key=your-key-here
```

### Review Output

The code review tool generates several artifacts:

1. `structure.md`: Detailed documentation of the code structure
2. `structure.json`: Machine-readable code structure data
3. `review.md`: Overall code review feedback
4. `{filename}.review.md`: File-specific suggestions for each source file

### Review Coverage

The tool analyzes:
- Package structure
- File organization
- Function definitions and relationships
- Type definitions and relationships
- Documentation completeness
- Code complexity
- Function calls and dependencies

### Example Review Output

```markdown
# Code Review Feedback

## Code Organization
- Clear package structure following Go conventions
- Good separation of concerns between packages
- Consider grouping related functionality in 'internal' package

## Documentation
- Most functions well documented
- Missing package documentation in 'api' package
- Consider adding more examples in documentation

## Suggestions
1. Add context.Context parameter to key functions
2. Implement proper error wrapping
3. Add more unit tests for edge cases
4. Consider using interfaces for better abstraction
```

## Server Features

### Agent System
- **Specialized Agents**: Pre-configured agents for different types of tasks
  - File System Agent: File operations and management
  - API Agent: HTTP/REST API interactions
  - Text Processor Agent: Text analysis and manipulation
  - Default Agent: General-purpose task handling
- **Dynamic Agent Selection**: Semantic matching to choose the most appropriate agent
- **Agent Chaining**: Combine multiple agents for complex tasks
- **State Management**: Track agent context and history

### Task Execution
- **File System Operations**: 
  - Read, write, delete files
  - List directory contents
  - Create directories
  - Move and copy files
- **API Operations**:
  - HTTP methods (GET, POST, PUT, DELETE)
  - Custom headers and authentication
  - Response validation
  - WebSocket support
  - GraphQL integration
- **Error Handling**: Comprehensive error tracking and recovery

## Server Usage

### Starting the Server

```bash
go run main.go
```

The server provides the following endpoints:
- `/thoughtchain`: Main endpoint for task execution
- `/health`: Health check endpoint
- `/agents`: List available agents

### Making Requests

#### File Operations
```bash
curl -X POST http://localhost:8080/thoughtchain \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Create a configuration file and write some settings",
    "tasks": [
      {
        "type": "filesystem",
        "operation": "write",
        "path": "config/settings.json",
        "body": "eyJzZXR0aW5ncyI6eyJkZWJ1ZyI6dHJ1ZX19"
      }
    ]
  }'
```

#### API Operations
```bash
curl -X POST http://localhost:8080/thoughtchain \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Fetch user data and save to file",
    "tasks": [
      {
        "type": "api",
        "method": "GET",
        "url": "https://api.example.com/users",
        "headers": {
          "Authorization": "Bearer token123"
        },
        "valid_status": [200]
      },
      {
        "type": "filesystem",
        "operation": "write",
        "path": "data/users.json",
        "body": "eyJ1c2VycyI6W119"
      }
    ]
  }'
```

### Response Format

```json
{
  "thought_cloud": [
    "Analyzing task requirements and selecting appropriate agent",
    "Validating file path and permissions",
    "Preparing to write configuration data"
  ],
  "sequence": [
    "FileSystem operation: write on path: config/settings.json"
  ],
  "task_results": [
    {
      "description": "Write configuration file",
      "success": true
    }
  ],
      "error": "failed to execute request: connection refused"
    }
  ]
}
```

## Logging

Logs are written to `thoughtchain.log` and include:
- Request/response information
- Agent selection decisions
- Task execution details
- Error tracking

## License

MIT License
