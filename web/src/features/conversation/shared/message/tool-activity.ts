/**
 * INPUT: Tool 名称与动态输入参数。
 * OUTPUT: 用户可读的工具标题、完整输入摘要与折叠态紧凑摘要。
 * POS: 工具执行块、过程摘要和 Composer 权限面的文本投影真相源。
 */
import type { TranslationKey } from "@/shared/i18n/messages";

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
  goal_contract: "读取 Goal 命令契约",
  execution_contract: "读取工作图命令契约",
  get_execution: "读取工作图",
  prepare_plan_execution: "封存计划提案",
  plan_execution: "提交计划提案",
  abandon_execution: "终止当前执行",
  assign_work: "指派工作项",
  submit_work: "提交交付物",
  review_work: "验收工作项",
  block_work: "标记工作阻塞",
  resume_work: "恢复工作项",
  take_over_work: "接管工作项",
  audit_execution_alignment: "审计执行对齐",
  promote_execution_to_goal: "升级为 Goal",
  distill_workgraph: "保存工作图草图",
  distill_workgraph_workflow: "保存工作图草图",
  get_goal: "读取 Goal",
  create_goal: "创建 Goal",
  retarget_goal: "调整 Goal 目标",
  audit_objective_alignment: "审计 Goal 对齐",
  update_goal: "更新 Goal 状态",
};

const TOOL_TITLE_KEY_MAP: Readonly<Record<string, TranslationKey>> = {
  AskUserQuestion: "message.tool_ask_user_question",
  Bash: "message.tool_bash",
  Edit: "message.tool_edit",
  Glob: "message.tool_glob",
  Grep: "message.tool_grep",
  LS: "message.tool_ls",
  MultiEdit: "message.tool_multi_edit",
  Read: "message.tool_read",
  Skill: "message.tool_skill",
  Task: "message.tool_task",
  TodoWrite: "message.tool_todo_write",
  WebFetch: "message.tool_web_fetch",
  WebSearch: "message.tool_web_search",
  Write: "message.tool_write",
  get_execution: "message.tool_execution_get",
  prepare_plan_execution: "message.tool_execution_prepare_plan",
  plan_execution: "message.tool_execution_commit_plan",
  abandon_execution: "message.tool_execution_abandon",
  assign_work: "message.tool_execution_assign_work",
  submit_work: "message.tool_execution_submit_work",
  review_work: "message.tool_execution_review_work",
  block_work: "message.tool_execution_block_work",
  resume_work: "message.tool_execution_resume_work",
  take_over_work: "message.tool_execution_take_over_work",
  audit_execution_alignment: "message.tool_execution_audit_alignment",
  promote_execution_to_goal: "message.tool_execution_promote_to_goal",
  distill_workgraph: "message.tool_execution_distill_workflow",
  distill_workgraph_workflow: "message.tool_execution_distill_workflow",
  get_goal: "message.tool_goal_get",
  create_goal: "message.tool_goal_create",
  retarget_goal: "message.tool_goal_retarget",
  audit_objective_alignment: "message.tool_goal_audit_alignment",
  update_goal: "message.tool_goal_update",
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

export function getToolTitle(toolName: string, input?: unknown): string {
  const semanticToolName = getSemanticToolName(toolName, input);
  return TOOL_TITLE_MAP[semanticToolName] ?? TOOL_TITLE_MAP[toolName] ?? toolName;
}

export function getToolTitleKey(toolName: string): TranslationKey | null {
  return TOOL_TITLE_KEY_MAP[getSemanticToolName(toolName)] ?? null;
}

export function getLocalizedToolTitle(
  toolName: string,
  t: (key: TranslationKey) => string,
  input?: unknown,
): string {
  const semanticToolName = getSemanticToolName(toolName, input);
  const titleKey = TOOL_TITLE_KEY_MAP[semanticToolName];
  return titleKey ? t(titleKey) : getToolTitle(semanticToolName);
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

const RUNTIME_COMMAND_PATTERN = /^\s*(?:"\$\{NEXUS_COMMAND_PATH\}"|&\s+"\$\{env:NEXUS_COMMAND_PATH\}")\s+--json\s+(goal|execution)\s+(contract|inspect|invoke)(?:\s|$)/;
const RUNTIME_COMMAND_OPERATION_PATTERN = /(?:^|\s)--operation\s+([a-z][a-z0-9_]*)(?:\s|$)/;

export function getSemanticToolName(toolName: string, input?: unknown): string {
  if (toolName !== "Bash" && toolName !== "PowerShell") {
    return toolName;
  }
  const command = getStringField(asRecord(input), "command");
  if (!command) {
    return toolName;
  }
  const invocation = command.match(RUNTIME_COMMAND_PATTERN);
  if (!invocation) {
    return toolName;
  }
  const [, domain, action] = invocation;
  if (action === "contract") {
    return `${domain}_contract`;
  }
  if (action === "inspect") {
    return domain === "goal" ? "get_goal" : "get_execution";
  }
  return command.match(RUNTIME_COMMAND_OPERATION_PATTERN)?.[1] ?? toolName;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function getStringField(
  record: Record<string, unknown> | null,
  key: string,
): string | null {
  if (!record) return null;
  const value = record[key];
  return typeof value === "string" && value ? value : null;
}
