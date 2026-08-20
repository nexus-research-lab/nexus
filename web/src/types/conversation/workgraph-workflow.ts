/**
 * INPUT: internal/protocol/workgraph_workflow.go 的 owner-safe JSON。
 * OUTPUT: 后台模型抽取的临时草图与用户确认保存的命名 Slash 工作图展示类型。
 * POS: WorkGraph 草图预览、目录与复用 UI 的协议镜像；不包含运行事实。
 */

import type { ExecutionWorkItemKind } from "./execution";

export type WorkGraphWorkflowNodeRole = "auto" | "key" | "collaboration";

export interface WorkGraphWorkflowNode {
  logical_key: string;
  source_work_item_id: string;
  role: WorkGraphWorkflowNodeRole;
  kind: ExecutionWorkItemKind;
  subject: string;
  objective: string;
  deliverable: string;
  acceptance_criteria?: string[];
  required: boolean;
  terminal?: boolean;
  parent_logical_key?: string;
  position: number;
}

export interface WorkGraphWorkflowDependency {
  logical_key: string;
  depends_on_logical_key: string;
  kind: "hard" | "soft";
}

export interface WorkGraphWorkflow {
  id: string;
  slash_name: string;
  title: string;
  description?: string;
  source_execution_id: string;
  source_session_key: string;
  objective: string;
  completion_criteria?: string[];
  nodes: WorkGraphWorkflowNode[];
  dependencies?: WorkGraphWorkflowDependency[];
  version: number;
  created_at: string;
  updated_at: string;
}

export interface WorkGraphWorkflowPreview {
  preview_id: string;
  slash_name: string;
  title: string;
  description?: string;
  source_execution_id: string;
  source_session_key: string;
  objective: string;
  completion_criteria?: string[];
  nodes: WorkGraphWorkflowNode[];
  dependencies?: WorkGraphWorkflowDependency[];
  expires_at: string;
}

export interface WorkGraphWorkflowSaveReceipt {
  preview_id: string;
  status: "scheduled";
}
