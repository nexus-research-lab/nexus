package workspace

var defaultWorkspaceTemplates = map[string]string{
	"agents": `# AGENTS.md

## Role

- Purpose:
- Responsibilities:
- Out of scope:
- Preferred working style:

## Baseline Rules

- Follow the injected Agent Identity, Agent Profile, and this file first for role-specific behavior.
- Keep file and shell work inside WORKING DIRECTORY unless the user explicitly gives another safe path.
- Inspect the real source of truth before making claims about files, logs, databases, APIs, tools, or prior work.
- Use NEXUSCTL_COMMAND_PATH for Nexus CLI work when it is available. Do not search for cmd/nexusctl or construct go run ./cmd/nexusctl manually.
- Do not invent facts, tool results, files, links, or completed actions.
- If a request is ambiguous but inspectable, inspect first. Ask only when acting would be risky.
`,
	"user": `# USER.md

setup_status: unconfigured

## Setup Required

This file is the user's durable profile. It starts as a setup template.

On the first natural interaction, briefly introduce yourself and ask for the user's profile:

- Name and preferred name
- Preferred language
- Contact / platform IDs
- Stable collaboration preferences

After the user provides enough details, replace this entire file with a configured profile. Set setup_status to configured. Do not keep this setup guide after configuration.

## User Profile

- Name:
- Preferred name:
- Preferred language:
- Contact / platform IDs:

## Preferences

- Reply style:
- Disliked phrases:
- Current focus:

## After Setup

Replace this template instead of appending below it.
`,
	"soul": `# SOUL.md

## Personality

-

## Tone

-

## Emotion

-
`,
	"tools": `# TOOLS.md

## Tool Notes

-

## Skill Notes

-

## Constraints

-
`,
}

var mainAgentWorkspaceTemplates = map[string]string{
	"user": defaultWorkspaceTemplates["user"],
}
