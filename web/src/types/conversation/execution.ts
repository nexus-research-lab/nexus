/**
 * INPUT: internal/protocol/execution_view.go 的 JSON 协议。
 * OUTPUT: DM/Room 共用的安全 WorkGraph 展示类型。
 * POS: Execution 后端投影在 Web 端的跨会话协议镜像。
 */

export type ExecutionStatus =
  | "active"
  | "waiting"
  | "paused"
  | "completed"
  | "failed"
  | "cancelled"
  | "superseded";

export type ExecutionWorkItemStatus =
  | "waiting"
  | "ready"
  | "assigned"
  | "running"
  | "blocked"
  | "submitted"
  | "changes_requested"
  | "accepted"
  | "failed"
  | "cancelled";

export type ExecutionWorkItemKind =
  | "produce"
  | "review"
  | "verify"
  | "integrate";

export interface ExecutionPlanView {
  id: string;
  revision: number;
  status: "proposed" | "active" | "superseded" | "cancelled";
  revision_reason?: string;
  created_at: string;
  activated_at?: string | null;
}

export interface ExecutionProgressView {
  total: number;
  required: number;
  accepted: number;
  running: number;
  blocked: number;
  submitted: number;
  ready: number;
  waiting: number;
  changes_requested: number;
  failed: number;
  cancelled: number;
}

export interface ExecutionOutputScope {
  scope: string;
  mode: "exclusive" | "shared";
}

export interface ExecutionAttemptView {
  id: string;
  assignment_id: string;
  parent_attempt_id?: string;
  executor_kind: "agent" | "subagent";
  executor_agent_id?: string;
  parent_agent_id?: string;
  agent_round_id?: string;
  child_session_id?: string;
  task_id?: string;
  tool_use_id?: string;
  status:
    | "pending"
    | "running"
    | "succeeded"
    | "failed"
    | "interrupted"
    | "cancelled"
    | "timed_out";
  failure_reason?: string;
  created_at: string;
  started_at?: string | null;
  finished_at?: string | null;
}

export type ExecutionGraphNodeKind = "agent" | "subagent" | "tool" | "gate";

export type ExecutionGraphNodeVisibility = "primary" | "nested" | "detail";

export type ExecutionGraphEdgeKind =
  | "dependency"
  | "dispatch"
  | "coordination"
  | "spawn"
  | "invoke"
  | "guard"
  | "review"
  | "loop_back"
  | "retry";

export interface ExecutionGraphArtifactView {
  id?: string;
  type: "workspace_file_artifact";
  path: string;
  display_path?: string;
  label?: string;
  title?: string;
  artifact_kind?: string;
  mime_type?: string;
  operation?: string;
  scope?: "agent_workspace";
  workspace_agent_id?: string;
  source_tool_use_id?: string;
  source_tool_name?: string;
}

export interface ExecutionGraphNodeRunView {
  id: string;
  attempt_id?: string;
  runtime_node_id?: string;
  agent_round_id?: string;
  subject_id?: string;
  status?: string;
  result_summary?: string;
  error_code?: string;
  error_summary?: string;
  summary_truncated?: boolean;
  duration_ms?: number;
  started_at?: string;
  finished_at?: string;
  artifacts?: ExecutionGraphArtifactView[];
}

export interface ExecutionGraphNodeView {
  id: string;
  kind: ExecutionGraphNodeKind;
  visibility: ExecutionGraphNodeVisibility;
  work_item_id: string;
  attempt_id?: string;
  parent_node_id?: string;
  agent_id?: string;
  agent_round_id?: string;
  subject_id?: string;
  name?: string;
  description?: string;
  lifecycle_status?: string;
  review_dispatch_id?: string;
  reviewer_kind?: "agent" | "user" | "system" | "policy";
  responsibility_status?: ExecutionWorkItemStatus;
  run_status?: ExecutionAttemptView["status"];
  result_summary?: string;
  error_code?: string;
  error_summary?: string;
  summary_truncated?: boolean;
  duration_ms?: number;
  started_at?: string;
  finished_at?: string;
  runs?: ExecutionGraphNodeRunView[];
  position: number;
}

export interface ExecutionGraphEdgeView {
  id: string;
  kind: ExecutionGraphEdgeKind;
  source_node_id: string;
  target_node_id: string;
  source_node_run_id?: string;
  target_node_run_id?: string;
  created_at?: string;
}

export interface ExecutionGraphView {
  nodes?: ExecutionGraphNodeView[];
  edges?: ExecutionGraphEdgeView[];
  runtime_node_total: number;
  runtime_edge_total: number;
  runtime_nodes_truncated: boolean;
  runtime_edges_truncated: boolean;
}

export interface ExecutionSubmissionView {
  id: string;
  submitter_agent_id: string;
  result_summary: string;
  result_refs?: string[];
  evidence?: string[];
  created_at: string;
}

export interface ExecutionCriterionResult {
  criterion: string;
  passed: boolean;
  evidence?: string[];
  note?: string;
}

export interface ExecutionAcceptanceView {
  id: string;
  decision: "accepted" | "rejected" | "changes_requested";
  reviewer_kind: "agent" | "user" | "system" | "policy";
  reviewer_id?: string;
  criteria_results?: ExecutionCriterionResult[];
  feedback?: string;
  created_at: string;
}

export interface ExecutionWorkItemView {
  id: string;
  logical_key: string;
  kind: ExecutionWorkItemKind;
  subject: string;
  objective: string;
  deliverable: string;
  acceptance_criteria?: string[];
  input_refs?: string[];
  output_scopes?: ExecutionOutputScope[];
  dependency_ids?: string[];
  parent_work_item_id?: string;
  required: boolean;
  terminal?: boolean;
  position: number;
  status: ExecutionWorkItemStatus;
  block_reason?: string;
  needed_input?: string;
  owner_agent_id?: string;
  assignment_id?: string;
  assignment_status?:
    | "assigned"
    | "active"
    | "released"
    | "completed"
    | "cancelled"
    | "revoked";
  assignment_strategy?: "self" | "room_member";
  review_agent_id?: string;
  review_dispatch_id?: string;
  review_status?: string;
  attempts?: ExecutionAttemptView[];
  submission?: ExecutionSubmissionView;
  acceptance?: ExecutionAcceptanceView;
  updated_at: string;
}

export interface ExecutionView {
  id: string;
  session_key: string;
  scope_kind: "dm" | "room";
  room_id?: string;
  conversation_id?: string;
  coordinator_agent_id?: string;
  objective: string;
  completion_criteria?: string[];
  goal_id?: string;
  goal_objective_revision?: number;
  status: ExecutionStatus;
  version: number;
  plan?: ExecutionPlanView;
  progress: ExecutionProgressView;
  work_items?: ExecutionWorkItemView[];
  graph: ExecutionGraphView;
  completion_blockers?: string[];
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
}
