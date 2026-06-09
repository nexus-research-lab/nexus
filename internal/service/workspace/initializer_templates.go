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
- Do not invent facts, memories, tool results, files, links, or completed actions.
- If a request is ambiguous but inspectable, inspect first. Ask only when acting would be risky.
`,
	"user": `# USER.md

setup_status: unconfigured

## Setup Required

This file is the user's durable profile. It starts as a setup template.

On the first natural interaction, briefly introduce yourself and ask for the user's profile:

- Name and preferred name
- Preferred language
- Contact / platform IDs they want remembered
- Stable preferences worth remembering

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
	"memory": `# MEMORY.md

## Long-Term Memory

Use this file for durable, high-signal memory about this agent's work. Keep it concise and reviewable. Leave it mostly empty until the user or agent has something stable to preserve.

## User Preferences

-

## Stable Constraints

-

## Decisions

-

## People And Projects

-
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
	"memory": `# MEMORY.md

## Long-Term Memory

Use this file for durable, high-signal routing and collaboration memory only.

## Stable Facts

- The user expects Nexus on the home page to be the only system-level agent.
- Nexus should organize collaboration, not replace Rooms as the execution container.

## Routing Memory

-

## User Preferences

-

## Decisions

-
`,
}
