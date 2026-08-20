/**
 * INPUT: exact ExecutionView、Slash 元数据与用户节点角色选择。
 * OUTPUT: 规范命令名和可见的 Skill + Nexus CLI 沉淀请求。
 * POS: 沉淀弹窗的纯模型层；不访问浏览器、HTTP 或 Workflow 存储。
 */

import type { ExecutionView } from "@/types/conversation/execution";
import type { WorkGraphWorkflowNodeRole } from "@/types/conversation/workgraph-workflow";

export interface WorkGraphDistillationSelection {
  enabled: boolean;
  role: WorkGraphWorkflowNodeRole;
}

export function normalizeWorkGraphWorkflowSlashName(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/^\/+/, "")
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64);
}

export function buildDistillationPrompt({
  description,
  execution,
  selections,
  slashName,
  title,
}: {
  description: string;
  execution: ExecutionView;
  selections: Map<string, WorkGraphDistillationSelection>;
  slashName: string;
  title: string;
}): string {
  const selected = (execution.work_items ?? [])
    .filter((item) => selections.get(item.id)?.enabled)
    .map((item) => {
      const role = selections.get(item.id)?.role ?? "key";
      return `- work_item_id=${item.id}; logical_key=${item.logical_key}; role=${role}; subject=${item.subject}`;
    })
    .join("\n");
  return [
    "请使用 execution-orchestrator Skill 和受管 Nexus CLI，把下面这张历史 WorkGraph 沉淀为可跨 Session 复用的命名 Slash workflow。",
    `源 Execution：${execution.id}`,
    `命令名：/${slashName}`,
    `标题：${title}`,
    description.trim() ? `适用说明：${description.trim()}` : "",
    "请先用 execution inspect --execution-id 读取 exact 源图，复核以下候选是否形成最小闭合责任子图；只保留 Work Item 的语义契约与依赖。不要保存工具调用、运行身份、Assignment、Attempt、结果、Artifact、Submission、Review 或 Acceptance。",
    "候选节点：",
    selected,
    "然后读取 distill_workgraph_workflow 的 fresh contract，并通过 nexus execution invoke 完成保存。",
  ].filter(Boolean).join("\n\n");
}
