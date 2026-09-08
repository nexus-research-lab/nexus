/**
 * INPUT: 服务端已保存命令和用户确认的草稿内容。
 * OUTPUT: 忽略存储顺序、版本编号和有效期的完整语义一致性判断。
 * POS: 保存回执丢失后的只读核对；不自动重放保存。
 */
import type { WorkGraphWorkflow, WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";

function content(value: WorkGraphWorkflow | WorkGraphWorkflowPreview) {
  return {
    slash_name: value.slash_name, title: value.title, description: value.description ?? "",
    source_execution_id: value.source_execution_id, source_session_key: value.source_session_key,
    objective: value.objective, completion_criteria: value.completion_criteria ?? [],
    nodes: value.nodes.map((node) => ({
      logical_key: node.logical_key, source_work_item_id: node.source_work_item_id ?? "",
      role: node.role, kind: node.kind, subject: node.subject, objective: node.objective,
      deliverable: node.deliverable, acceptance_criteria: node.acceptance_criteria ?? [],
      required: node.required, terminal: node.terminal ?? false,
      parent_logical_key: node.parent_logical_key ?? "", position: node.position,
    })).sort((a, b) => a.logical_key.localeCompare(b.logical_key)),
    dependencies: (value.dependencies ?? []).map((dependency) => ({
      logical_key: dependency.logical_key, depends_on_logical_key: dependency.depends_on_logical_key, kind: dependency.kind,
    })).sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b))),
  };
}

export function savedWorkGraphMatchesPreview(workflow: WorkGraphWorkflow | WorkGraphWorkflowPreview | undefined, preview: WorkGraphWorkflowPreview): boolean {
  return workflow !== undefined && JSON.stringify(content(workflow)) === JSON.stringify(content(preview));
}
