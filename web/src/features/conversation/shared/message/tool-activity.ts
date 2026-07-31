/**
 * INPUT: Tool 名称与动态输入参数。
 * OUTPUT: 用户可读的工具标题、完整输入摘要与折叠态紧凑摘要。
 * POS: 工具执行块、过程摘要和 Composer 权限面的文本投影真相源。
 */
const TOOL_TITLE_MAP: Record<string, string> = {
  Bash: "执行命令",
  Read: "读取内容",
  Write: "写入内容",
  Edit: "修改内容",
  MultiEdit: "批量修改",
  Grep: "查找内容",
  Glob: "浏览文件",
  LS: "查看目录",
  TodoWrite: "更新计划",
  AskUserQuestion: "等待你的确认",
  WebSearch: "网络搜索",
  WebFetch: "抓取网页",
  Skill: "调用技能",
  Task: "委派任务",
  get_execution: "读取工作图",
  plan_execution: "建立执行计划",
  abandon_execution: "终止当前执行",
  assign_work: "指派工作项",
  submit_work: "提交交付物",
  review_work: "验收工作项",
  block_work: "标记工作阻塞",
  resume_work: "恢复工作项",
  take_over_work: "接管工作项",
  promote_execution_to_goal: "升级为 Goal",
};

const INPUT_SUMMARY_KEYS = [
  "file_path",
  "path",
  "url",
  "query",
  "pattern",
  "description",
  "task",
  "prompt",
  "objective",
  "logical_key",
  "result_summary",
  "reason",
] as const;

const COMMAND_SUMMARY_LIMIT = 50;

export function getToolTitle(toolName: string): string {
  const semanticToolName = getExecutionToolLeaf(toolName);
  return TOOL_TITLE_MAP[semanticToolName] ?? TOOL_TITLE_MAP[toolName] ?? toolName;
}

export function getToolInputSummary(input: unknown): string | null {
  const record = asRecord(input);
  if (!record) return null;

  for (const key of INPUT_SUMMARY_KEYS) {
    const value = getStringField(record, key);
    if (value) return value;
  }

  const command = getStringField(record, "command");
  return command ? formatCommandSummary(command) : null;
}

export function getCompactToolInputSummary(input: unknown): string | null {
  const record = asRecord(input);
  if (!record) {
    return null;
  }
  for (const key of ["file_path", "path"] as const) {
    const value = getStringField(record, key);
    if (value) {
      return getPathLeaf(value);
    }
  }
  return getToolInputSummary(input);
}

function getPathLeaf(value: string): string {
  const trimmed = value.trim().replace(/[\\/]+$/, "");
  return trimmed.split(/[\\/]/).at(-1) || value;
}

function formatCommandSummary(command: string): string {
  const suffix = command.length > COMMAND_SUMMARY_LIMIT ? "..." : "";
  return `$ ${command.slice(0, COMMAND_SUMMARY_LIMIT)}${suffix}`;
}

function getExecutionToolLeaf(toolName: string): string {
  const prefix = "mcp__nexus_execution__";
  return toolName.startsWith(prefix) ? toolName.slice(prefix.length) : toolName;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function getStringField(
  record: Record<string, unknown>,
  key: string,
): string | null {
  const value = record[key];
  return typeof value === "string" && value ? value : null;
}
