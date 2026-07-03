import type { ImageContent } from "@/types/conversation/message";
import type { PermissionUpdate } from "@/types/conversation/permission";

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
};

export const FIELD_LABEL_MAP: Record<string, string> = {
  query: "搜索内容",
  url: "网址",
  command: "命令",
  path: "路径",
  file_path: "文件路径",
  pattern: "匹配内容",
  prompt: "提示词",
  description: "说明",
  task: "任务",
  mode: "模式",
  directories: "目录",
  answers: "回答",
};

const PRIMARY_INPUT_KEYS = [
  "command",
  "query",
  "url",
  "path",
  "file_path",
  "pattern",
  "description",
  "prompt",
  "task",
] as const;

export const TOOL_TONE_STYLES: Record<string, string> = {
  default: "text-(--icon-muted)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--warning)",
};

export const TOOL_LABEL_STYLES: Record<string, string> = {
  default: "text-(--text-default)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--warning)",
};

export function getToolTitle(toolName: string): string {
  return TOOL_TITLE_MAP[toolName] ?? toolName;
}

export function formatPermissionValue(value: unknown): string {
  if (value == null || value === "") return "空";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  if (Array.isArray(value)) {
    return value.map((item) => formatPermissionValue(item)).join("、");
  }
  if (typeof value === "object") {
    return Object.entries(value as Record<string, unknown>)
      .map(([key, nestedValue]) => `${FIELD_LABEL_MAP[key] || key}：${formatPermissionValue(nestedValue)}`)
      .join("；");
  }
  return String(value);
}

export function getReadableSuggestions(suggestions: PermissionUpdate[] = []) {
  const destinationMap: Record<string, string> = {
    session: "仅本会话",
    projectSettings: "项目设置",
    userSettings: "用户设置",
    localSettings: "本地设置",
  };
  const behaviorMap: Record<string, string> = {
    allow: "允许",
    deny: "拒绝",
    ask: "继续询问",
  };

  return suggestions.map((suggestion, index) => {
    const destination = suggestion.destination
      ? destinationMap[suggestion.destination] || suggestion.destination
      : "当前会话";
    const behavior = suggestion.behavior
      ? behaviorMap[suggestion.behavior] || suggestion.behavior
      : "更新规则";

    return {
      index,
      label: behavior === "允许"
        ? `写入${destination}`
        : `${behavior}并写入${destination}`,
    };
  });
}

export function getInputSummary(input: any): string | null {
  if (!input) return null;
  if (input.file_path) return input.file_path;
  if (input.path) return input.path;
  if (input.url) return input.url;
  if (input.query) return input.query;
  if (input.pattern) return input.pattern;
  if (input.description) return input.description;
  if (input.task) return input.task;
  if (input.prompt) return input.prompt;
  if (input.command) return `$ ${input.command.slice(0, 50)}${input.command.length > 50 ? "..." : ""}`;
  return null;
}

export function getPrimaryInputDetail(input: any): { key: string; value: string } | null {
  if (!input) return null;
  for (const key of PRIMARY_INPUT_KEYS) {
    const value = input[key];
    if (typeof value === "string" && value) {
      return { key, value };
    }
  }
  return null;
}

export function getResultSummary(content: any): string {
  if (typeof content === "string") {
    return content.slice(0, 80) + (content.length > 80 ? "..." : "");
  }
  return "JSON 数据";
}

export function isImageContent(value: unknown): value is ImageContent {
  return Boolean(
    value &&
    typeof value === "object" &&
    (value as { type?: unknown }).type === "image",
  );
}
