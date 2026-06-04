# Operation Stage Handoff - Tool Call to Desktop App Chain

## Why this handoff exists

The current operation stage work has drifted toward a visually rich demo, but the core product chain is still not strong enough. The next session should treat the stage as a real desktop workbench, not as a tool-log dashboard.

The user wants a new, cleaner session to continue from this point. This document is the canonical handoff for that session.

## Current branch and workspace

- Repo: `/Users/berhand/program/Work/Nexus/nexus-core`
- Active feature worktree used in the previous session: `/private/tmp/nexus-operation-stage`
- Branch: `codex/operation-stage`
- Remote branch: `origin/codex/operation-stage`
- Latest pushed commit at handoff time: `37b3d4ac :sparkles: 优化舞台 Safari 起始页`

Important: the previous session has two uncommitted Dock empty-state WIP files in `/private/tmp/nexus-operation-stage`:

- `web/src/features/conversation/operation/stage/operation-stage-dock-launch.ts`
- `web/scripts/verify-operation-stage-projector.mjs`

Those changes are not the next priority. They can be reviewed later, stashed, committed separately, or discarded only if the user explicitly approves. The next priority is the core tool-call-to-window chain.

## Product direction

The stage should feel like Nexus is operating a computer.

It should not feel like:

- a runtime debug board
- a collection of status cards
- a demo collage of overlapping panels
- a generic tool execution timeline

The stage should feel like:

- a quiet macOS-like desktop when idle
- a computer waking into action when the user starts a task
- apps opening because tool calls require them
- app windows updating as tool results arrive
- a finished desktop state that shows the created artifact, preview, terminal output, and handoff summary naturally

## Core chain to build first

Focus on this chain before Dock polish:

```text
NexusOperationEvent stream
  -> classify event as desktop intent
  -> update app session model
  -> open/focus/update mac-like app window
  -> render real content for that app
  -> preserve final desktop state
```

The critical behavior is not "one event equals one panel". It is "one tool call changes the desktop state".

Examples:

- `Read`, `Glob`, `Grep`, `LS` should open or update Finder/Code-style file inspection.
- `Write`, `Edit`, `MultiEdit` should open or update a Code editor window with the actual target file and diff/content.
- `Bash` should open Terminal and show command, stdout/stderr, exit code, and running/done/error state.
- `web_search`, `fetch`, browser-like tools should open Safari/browser with query, URL, or result pages.
- HTML preview/open should open Safari or Preview with the generated artifact, not just a text summary.
- Final handoff should be a lightweight delivery window plus desktop artifacts, not a dashboard.

## Current diagnosis

The current implementation has useful pieces, but the main abstraction is wrong for the desired experience.

Observed problems:

- The stage still often renders "event windows" rather than persistent app sessions.
- Tool categorization exists in pieces, but it does not consistently drive app behavior.
- Terminal still does not feel like a real terminal unless command output is rendered as a transcript.
- Browser/search/HTML preview behavior needs to map to a browser window, not a generic result card.
- Completion state can become visually crowded and demo-like.
- Dock and idle launches are improving, but they are secondary until the tool-to-window chain is stable.

## Existing relevant files

Start by reading these files:

- `web/src/features/conversation/operation/operation-projector.ts`
- `web/src/features/conversation/operation/operation-scene-planner.ts`
- `web/src/features/conversation/operation/operation-scene-window-policy.ts`
- `web/src/features/conversation/operation/operation-scene-generic-tool-window.ts`
- `web/src/features/conversation/operation/operation-tool-inference.ts`
- `web/src/features/conversation/operation/operation-tool-catalog.ts`
- `web/src/features/conversation/operation/operation-terminal-lines.ts`
- `web/src/features/conversation/operation/operation-desktop-types.ts`
- `web/src/features/conversation/operation/stage/operation-stage-desktop.tsx`
- `web/src/features/conversation/operation/stage/operation-stage-window-position.ts`
- `web/src/features/conversation/operation/apps/operation-app-renderers.tsx`
- `web/src/features/conversation/operation/apps/terminal-session.tsx`
- `web/src/features/conversation/operation/apps/browser-surface.tsx`
- `web/src/features/conversation/operation/apps/code-editor-session.ts`
- `web/src/features/conversation/operation/apps/workspace-finder-surface.tsx`
- `web/src/dev/operation-stage-preview.tsx`
- `web/scripts/verify-operation-stage-projector.mjs`

Do not start by polishing Dock styling. First trace how an event becomes a window and how the window content is chosen.

## Suggested next implementation plan

1. Add or extract a desktop intent layer.

   Suggested shape:

   ```ts
   type StageDesktopIntent =
     | { app: "finder"; action: "inspect_files"; event_id: string; target?: string }
     | { app: "code"; action: "edit_file"; event_id: string; target?: string }
     | { app: "terminal"; action: "run_command"; event_id: string; command?: string }
     | { app: "browser"; action: "browse"; event_id: string; url?: string; query?: string }
     | { app: "preview"; action: "preview_artifact"; event_id: string; target?: string }
     | { app: "handoff"; action: "summarize_delivery"; event_id: string };
   ```

   This should be derived from the SDK-facing `NexusOperationEvent` data, not from hard-coded preview labels.

2. Build persistent app sessions from intents.

   The stage should know whether Terminal, Code, Browser, Finder, Preview, and Handoff already exist, and update/focus them instead of creating unrelated panels.

3. Make Terminal real first.

   A `Bash` event should render:

   - command prompt
   - command text
   - streaming/running marker if phase is running
   - stdout lines
   - stderr/error lines
   - exit code
   - cwd/target if available

   This is the most obvious gap to users.

4. Make browser/HTML preview real second.

   Web search/fetch/open HTML should render in a Safari-like surface:

   - address/search bar
   - page/result title
   - result list or local artifact preview
   - loading/done/error state

5. Rework final state.

   The final state should preserve the last meaningful desktop arrangement:

   - created file visible in Finder or on desktop
   - Code window showing the artifact/source
   - Terminal with final verification output if a command ran
   - Safari/Preview with generated artifact if available
   - small Handoff window summarizing what is ready

   Avoid turning the stage into a report dashboard.

## Preview scenarios to keep using

`web/src/dev/operation-stage-preview.tsx` has staged examples:

- `idle`
- `write`
- `tool`
- `search`
- `permission`
- `open`
- `done`

Use it for visual checks, but do not let it become the only source of truth. The chain should be driven by realistic `NexusOperationEvent` payloads.

Suggested local check:

```bash
pnpm --dir web typecheck
pnpm --dir web lint
pnpm --dir web verify:operation-stage
```

If running visually:

```bash
pnpm --dir web dev --host 127.0.0.1 --port 3008
```

Then open:

```text
http://127.0.0.1:3008/operation-stage-preview.html?step=done
```

Always stop the dev server before finishing the session.

## User expectations

The user cares about:

- real interface inspection with screenshots, not code-only changes
- files staying reasonably small, roughly around 500 lines unless a larger file is justified
- clear module boundaries
- coherent commits and pushes
- no destructive cleanup of user changes
- Chinese communication, concise but grounded in repo state

The user explicitly said the Dock can wait. The next session should acknowledge that and work on the real event chain first.

## Starter prompt for the next session

Use this prompt in the new session:

```text
继续 Nexus operation stage 工作。请先阅读 docs/specs/2026-06-04-operation-stage-handoff.md。

目标：不要继续优先修 Dock。先打通工具调用 -> 桌面意图 -> App session -> macOS-like 窗口打开/聚焦/内容更新这条核心链路。尤其先让 Bash/Terminal 显示真实命令输出，再让 web_search/fetch/open html 映射到 Safari/浏览器窗口。

分支应基于 codex/operation-stage。开始前请检查当前 worktree/branch/status，保护未提交改动。实现后用 typecheck/lint/verify-operation-stage 和浏览器截图验证。
```
