# ThoughtChain

ThoughtChain is a sophisticated Go library for executing sequences of tasks using specialized agents powered by LLM-based thought generation. It provides a flexible and extensible system for handling various types of operations including filesystem tasks, API interactions, and text processing.

## Prerequisites

1. Go 1.16 or higher
2. OpenRouter API Key (Get one from [OpenRouter](https://openrouter.ai/))

## Installation

```bash
go get github.com/yourusername/thoughtchain
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

## Running the Server

```bash
go run main.go
```

The server will start on port 8080 by default. You can change the port by setting the PORT environment variable:
```bash
PORT=3000 go run main.go
```

## Features

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

## Usage Examples

### File Operations
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

### API Operations
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
  "conclusion": "Successfully created and wrote to configuration file"
}
```

## Development

### Running Tests
```bash
go test ./... -v
```

### Adding New Agents

1. Create a new agent configuration file in `config/agents`:
```json
{
  "name": "custom_agent",
  "description": "Custom agent for specific tasks",
  "prompt_templates": {
    "default": "Process the following task: {{.Task}}"
  },
  "capabilities": ["custom_operation"],
  "required_context": [],
  "chains_with": ["file_agent"],
  "max_tokens": 1000,
  "temperature": 0.7
}
```

2. The agent will be automatically loaded when the server starts.

### Architecture

1. **Agent Manager**: Handles agent loading and lifecycle
2. **Semantic Selector**: Chooses appropriate agents for tasks
3. **Prompt Configurator**: Manages prompt generation and templating
4. **Task Executor**: Handles task execution and error management
5. **State Manager**: Maintains agent state and history

### Flow

1. Request received → Parse task description
2. Select appropriate agent(s) based on task
3. Configure prompt using agent's template
4. Generate thoughts using LLM
5. Execute task sequence
6. Track results and update state
7. Generate conclusion
8. Return response

## Endpoints

- `POST /thoughtchain`: Execute thought chains
- `GET /health`: Health check endpoint
- `GET /agents`: List available agents

## Error Handling

The system provides detailed error information in the response:
```json
{
  "task_results": [
    {
      "description": "API call: GET https://api.example.com/users",
      "success": false,
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
