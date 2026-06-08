# Operation Tool Visual Components

## Current Tool Inventory

Operation stage currently recognizes these tool names through `operation-tool-catalog.ts`:

- Workspace navigation: `Glob`, `Grep`, `LS`
- Workspace read: `Read`
- Workspace write: `Write`, `Edit`, `MultiEdit`, `NotebookEdit`
- Command execution: `Bash`, `KillShell`
- Web browsing/research: `WebSearch`, `WebFetch`
- Task and plan tracking: `Task`, `TaskOutput`, `TodoWrite`, `EnterPlanMode`, `ExitPlanMode`
- Knowledge/tool context: `Skill`
- Human gate: `AskUserQuestion`
- Fallback: any inferred or unknown tool name

The source of truth for visual grouping is `operation-tool-visual-contract.ts`.

## Visual Component Groups

| Group | Tools | Primary component | User-visible interaction |
| --- | --- | --- | --- |
| Workspace navigation | `Glob`, `Grep`, `LS` | Finder | Browse directories, search files, select artifacts |
| Workspace reader | `Read` | Code reader | Open a file and scan its content |
| Workspace writer | `Write`, `Edit`, `MultiEdit`, `NotebookEdit` | Code writer | Create/update files, stream typed content, show diff state |
| Command runner | `Bash`, `KillShell` | Terminal | Type commands, stream stdout/stderr, show exit state |
| Web browser | `WebSearch`, `WebFetch` | Safari | Load search results, fetched pages, or local HTML artifacts |
| Task planner | `Task`, `TaskOutput`, `TodoWrite`, `EnterPlanMode`, `ExitPlanMode` | Activity Monitor | Show task delegation, todo progress, and plan state |
| Knowledge tool | `Skill` | Nexus tool/knowledge viewer | Show skill context, docs snippets, and tool evidence |
| Human gate | `AskUserQuestion` plus permission events | System Settings | Let the user confirm, deny, or answer |
| Handoff | round summary | Delivery desk | Preserve final windows, artifacts, and next action |

## Shared Desktop Controls

Every primary component is hosted by `OperationStageWindow`, which owns the common desktop controls:

- close
- minimize
- zoom
- drag
- focus
- dock restore

Human gate components also need confirm and deny controls. Those controls are part of the visual contract, but the permission buttons still need to be wired to the real permission action path before they are considered product-complete.

## Implementation Priority

1. Code writer/reader: keep the file content itself as the main surface. No JSON cards. Write/Edit should stream typed content; Read should open and scan.
2. Terminal: command prompt, command text, stdout/stderr, exit status, and running cursor.
3. Safari: web search/fetch/open-html should become a browser window with address bar and rendered content/results.
4. Human gate: replace passive permission mock buttons with real confirm/deny actions.
5. Finder/task/knowledge/handoff: use the same window controls but keep the app body domain-specific.
