export const ASK_USER_QUESTION_TOOL_NAME = "AskUserQuestion";

const SUBAGENT_TOOL_NAMES = new Set(["Agent", "Task"]);

export function isSubagentToolName(toolName: string): boolean {
  return SUBAGENT_TOOL_NAMES.has(toolName.trim());
}
