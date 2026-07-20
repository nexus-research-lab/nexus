import type { OperationToolProfile } from "./operation-tool-catalog";

type OperationToolProfileKey =
  | "AskUserQuestion"
  | "Bash"
  | "Edit"
  | "EnterPlanMode"
  | "ExitPlanMode"
  | "Glob"
  | "Grep"
  | "KillShell"
  | "LS"
  | "NotebookEdit"
  | "Read"
  | "Skill"
  | "Task"
  | "TaskOutput"
  | "TodoWrite"
  | "WebFetch"
  | "WebSearch"
  | "Write";

/** nxs canonical names are explicit SDK contracts, not fuzzy compatibility guesses. */
const NXS_TOOL_PROFILE_KEYS: Readonly<Record<string, OperationToolProfileKey>> = {
  "agent.run": "Task",
  "browser.download": "WebFetch",
  "browser.open": "WebFetch",
  "filesystem.list": "LS",
  "filesystem.read": "Read",
  "filesystem.write": "Write",
  "notebook.edit": "NotebookEdit",
  "patch.apply": "Edit",
  "plan.enter": "EnterPlanMode",
  "plan.exit": "ExitPlanMode",
  "search.glob": "Glob",
  "search.grep": "Grep",
  "shell.kill": "KillShell",
  "shell.run": "Bash",
  "skill.invoke": "Skill",
  "skill.use": "Skill",
  "task.background": "Task",
  "task.output": "TaskOutput",
  "task.run": "Task",
  "todo.write": "TodoWrite",
  "user.ask": "AskUserQuestion",
  "web.extract": "WebFetch",
  "web.map": "WebSearch",
  "web.search": "WebSearch",
};

/** Some nxs transports preserve the OpenAI function namespace verbatim. */
const NXS_TRANSPORT_PROFILE_KEYS: Readonly<Record<string, OperationToolProfileKey>> = {
  "functions.Bash": "Bash",
  "functions.KillShell": "KillShell",
};

export function inferOperationToolProfile(
  tool_name: string,
  profiles: Record<string, OperationToolProfile>,
  _default_target_keys: readonly string[],
): OperationToolProfile | null {
  const normalized = tool_name.trim();
  const profile_key = NXS_TOOL_PROFILE_KEYS[normalized]
    ?? NXS_TRANSPORT_PROFILE_KEYS[normalized];
  return profile_key ? profiles[profile_key] ?? null : null;
}
