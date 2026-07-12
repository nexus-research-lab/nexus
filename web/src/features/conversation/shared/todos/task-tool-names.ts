export type TaskListToolName = "TaskCreate" | "TaskList" | "TaskUpdate";

export const TASK_LIST_TOOL_NAMES: TaskListToolName[] = [
  "TaskCreate",
  "TaskList",
  "TaskUpdate",
];

export const CONVERSATION_TASK_TOOL_NAMES = [
  "TodoWrite",
  ...TASK_LIST_TOOL_NAMES,
];

const TASK_LIST_TOOL_NAME_SET = new Set<string>(TASK_LIST_TOOL_NAMES);

export function isTaskListToolName(name: string): name is TaskListToolName {
  return TASK_LIST_TOOL_NAME_SET.has(name);
}
