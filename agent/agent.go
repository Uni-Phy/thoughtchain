package agent

// Agent represents a specialized agent with specific capabilities
type Agent struct {
    Name            string            `json:"name"`
    Description     string            `json:"description"`
    PromptTemplates map[string]string `json:"prompt_templates"`
    Capabilities    []string          `json:"capabilities"`
    RequiredContext []string          `json:"required_context"`
    ChainsWith      []string          `json:"chains_with"`
    MaxTokens       int               `json:"max_tokens"`
    Temperature     float64           `json:"temperature"`
    score           float64           // Define the score field here
}

// ExampleAgentConfigs provides example agent configurations
var ExampleAgentConfigs = map[string][]byte{
	"file_agent.json": []byte(`{
		"name": "file_agent",
		"description": "Specialized agent for file system operations",
		"prompt_templates": {
			"default": "You are a file system expert. Your task is: {{.Task}}\n\nContext:\n{{range $key, $value := .Context}}- {{$key}}: {{$value}}\n{{end}}\n{{if .PreviousTasks}}Previous tasks:\n{{range .PreviousTasks}}- {{.}}\n{{end}}{{end}}{{if .PreviousErrors}}Previous errors:\n{{range .PreviousErrors}}- {{.}}\n{{end}}{{end}}"
		},
		"capabilities": [
			"read_file",
			"write_file",
			"delete_file",
			"list_directory",
			"create_directory",
			"move_file",
			"copy_file"
		],
		"required_context": [
			"working_directory"
		],
		"chains_with": [
			"api_agent",
			"text_processor_agent"
		],
		"max_tokens": 2048,
		"temperature": 0.7
	}`),
	"api_agent.json": []byte(`{
		"name": "api_agent",
		"description": "Specialized agent for API interactions",
		"prompt_templates": {
			"default": "You are an API expert. Your task is: {{.Task}}\n\nContext:\n{{range $key, $value := .Context}}- {{$key}}: {{$value}}\n{{end}}\n{{if .PreviousTasks}}Previous tasks:\n{{range .PreviousTasks}}- {{.}}\n{{end}}{{end}}{{if .PreviousErrors}}Previous errors:\n{{range .PreviousErrors}}- {{.}}\n{{end}}{{end}}"
		},
		"capabilities": [
			"http_get",
			"http_post",
			"http_put",
			"http_delete",
			"websocket",
			"graphql"
		],
		"required_context": [
			"api_base_url",
			"auth_token"
		],
		"chains_with": [
			"file_agent",
			"data_processor_agent"
		],
		"max_tokens": 2048,
		"temperature": 0.7
	}`),
	"text_processor_agent.json": []byte(`{
		"name": "text_processor_agent",
		"description": "Specialized agent for text processing and analysis",
		"prompt_templates": {
			"default": "You are a text processing expert. Your task is: {{.Task}}\n\nContext:\n{{range $key, $value := .Context}}- {{$key}}: {{$value}}\n{{end}}\n{{if .PreviousTasks}}Previous tasks:\n{{range .PreviousTasks}}- {{.}}\n{{end}}{{end}}{{if .PreviousErrors}}Previous errors:\n{{range .PreviousErrors}}- {{.}}\n{{end}}{{end}}"
		},
		"capabilities": [
			"text_analysis",
			"sentiment_analysis",
			"language_detection",
			"summarization",
			"translation"
		],
		"required_context": [
			"input_text",
			"target_language"
		],
		"chains_with": [
			"file_agent",
			"api_agent"
		],
		"max_tokens": 4096,
		"temperature": 0.8
	}`),
	"default_agent.json": []byte(`{
		"name": "default_agent",
		"description": "General-purpose agent for handling various tasks",
		"prompt_templates": {
			"default": "You are a helpful assistant. Your task is: {{.Task}}\n\nContext:\n{{range $key, $value := .Context}}- {{$key}}: {{$value}}\n{{end}}\n{{if .PreviousTasks}}Previous tasks:\n{{range .PreviousTasks}}- {{.}}\n{{end}}{{end}}{{if .PreviousErrors}}Previous errors:\n{{range .PreviousErrors}}- {{.}}\n{{end}}{{end}}"
		},
		"capabilities": [
			"general_assistance",
			"task_planning",
			"error_handling"
		],
		"required_context": [],
		"chains_with": [
			"file_agent",
			"api_agent",
			"text_processor_agent"
		],
		"max_tokens": 2048,
		"temperature": 0.7
	}`)}