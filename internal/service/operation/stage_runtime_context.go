// INPUT: Runtime session key and the current Operation Stage presence lease.
// OUTPUT: A hidden next-turn capability contract only while the Stage is visible.
// POS: Operation Stage runtime-awareness policy shared by DM and Room execution.
package operation

import (
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const stageRuntimeContextName = "operation_stage"

const stageRuntimeContext = `Nexus Operation Stage is open for this conversation. The user can watch supported tool activity through real Agent OS apps.

- Keep using the normal tools. Nexus projects Bash and KillShell into Terminal; workspace read/write/edit tools into Files, Editor, or previews; WebSearch, WebFetch, and opened web content into Navi; Skill tools into Library; and task tools into Tasks.
- Never claim that an app, file, page, command, or result is visible unless a corresponding real tool call produced it. The Stage controls window layout and focus.
- Terminal is an observable execution surface and does not provide interactive stdin to the user. Do not leave a user-facing workflow waiting for terminal input.
- For an interactive visual experience, prefer a self-contained HTML/CSS/JavaScript artifact in the current workspace. After it is ready, use one standalone host-open command such as open <workspace-relative-path-or-url>; Nexus will route it into the Stage instead of the user's external browser. Do not combine that opener with another shell command.
- Keep local artifacts inside the current workspace and prefer workspace-relative paths. Use presentation-oriented tools only when they help answer the request; do not create unnecessary files or tool calls merely to animate the Stage.
- Leave presented artifacts usable after the round so the user can inspect or interact with them.`

// StageRuntimeContext returns the per-turn Agent OS capability contract while the Stage is online.
func (s *Service) StageRuntimeContext(sessionKey string) []runtimectx.ContextualInputBlock {
	if s == nil || !s.IsStageActive(sessionKey) {
		return nil
	}
	normalizedSessionKey := strings.TrimSpace(sessionKey)
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock(
			stageRuntimeContextName,
			stageRuntimeContext,
			20,
			map[string]string{
				"session_key": normalizedSessionKey,
				"surface":     "operation_stage",
			},
		),
	}
}
