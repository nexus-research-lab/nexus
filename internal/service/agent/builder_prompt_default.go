package agent

import _ "embed"

//go:embed prompt_base.md
var defaultBaseSystemPrompt string

//go:embed prompt_main_agent.md
var defaultMainAgentSystemPrompt string
