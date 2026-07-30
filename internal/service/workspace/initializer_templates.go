package workspace

import agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"

var defaultWorkspaceTemplates = map[string]string{
	"agents": agentsvc.DefaultProfileTemplate(),
	"user": `setup_status: unconfigured

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
	"soul": `## Personality

-

## Tone

-

## Emotion

-
`,
	"tools": `## Tool Notes

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
