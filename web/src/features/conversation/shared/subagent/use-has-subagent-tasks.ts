// INPUT: 当前会话的子智能体任务来源（可缺省）。
// OUTPUT: 有任务返回 true；无来源、首次读取中或成功读取的空目录返回 false；读取失败且无快照返回 null（未知，由消费方保留入口以便查看恢复提示）。
// POS: Header 入口可见性的轻量投影；复用来源级任务目录与共享 WebSocket 失效，不另建轮询。
"use client";

import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { useSubagentTasks } from "./use-subagent-tasks";

export function useHasSubagentTasks(source: SubagentTaskSource | null): boolean | null {
  const { data, error } = useSubagentTasks(source, Boolean(source));
  if (!source) return false;
  if (!data) return error ? null : false;
  return data.items.length > 0;
}
