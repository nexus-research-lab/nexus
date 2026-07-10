import type {
  ChoiceDef,
  ExecutionKind,
  ExecutionMode,
  ReplyMode,
  TargetType,
} from "../scheduled-task-dialog-types";

export const TARGET_TYPE_OPTIONS: ChoiceDef<TargetType>[] = [
  { key: "agent", label: "智能体" },
  { key: "room", label: "Room" },
];

export const EXECUTION_KIND_OPTIONS: ChoiceDef<ExecutionKind>[] = [
  { key: "agent", label: "Agent 执行" },
  { key: "script", label: "脚本执行" },
];

export const EXECUTION_MODE_OPTIONS: ChoiceDef<ExecutionMode>[] = [
  { key: "main", label: "使用主会话" },
  { key: "existing", label: "使用现有会话" },
  { key: "temporary", label: "每次新建临时会话" },
  { key: "dedicated", label: "使用专用长期会话" },
];

export const REPLY_MODE_OPTIONS: ChoiceDef<ReplyMode>[] = [
  { key: "none", label: "不回传" },
  { key: "execution", label: "回到执行会话" },
  { key: "selected", label: "回到指定会话" },
];
