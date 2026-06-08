import type { WorkspaceActivityItem } from "@/types/app/workspace-live";
import type {
  Message,
  SystemEventContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message";
import type { PendingPermission } from "@/types/conversation/permission";

import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationEvidence,
  OperationPhase,
} from "./operation-types";
import {
  collect_recent_operation_evidence,
  filter_workspace_events_for_stage,
  pick_operation_active_event,
  resolve_workspace_event_round_id,
} from "./operation-projection-timeline";
import {
  build_operation_message_runtime_events,
  build_workspace_runtime_event,
  sort_operation_runtime_events,
} from "./operation-runtime-event-stream";
import {
  DEFAULT_TARGET_KEYS,
  extract_operation_input_value,
  resolve_operation_tool_profile,
} from "./operation-tool-catalog";
import {
  collect_unresolved_tool_use_candidates,
  filter_pending_permissions_for_stage,
  match_pending_permissions_to_tool_uses,
} from "./operation-pending-permissions";
import {
  OPERATION_MAX_TEXT_PREVIEW,
  redact_projected_value,
  summarize_projected_value,
  truncate_projected_text,
} from "./operation-projection-preview";
import { project_result_summary_event } from "./operation-summary-events";

const MAX_EVENTS = 24;
const MAX_EVIDENCE = 8;
const MAX_PROJECTED_MESSAGES = 80;

interface ProjectOperationSnapshotParams {
  key: string;
  session_key: string | null;
  agent_id?: string | null;
  messages: Message[];
  pending_permissions: PendingPermission[];
  live_round_ids: string[];
  workspace_events: WorkspaceActivityItem[];
}

export function project_operation_snapshot({
  key,
  session_key,
  agent_id,
  messages,
  pending_permissions,
  live_round_ids,
  workspace_events,
}: ProjectOperationSnapshotParams): NexusOperationSnapshot {
  const projected_messages = messages.slice(-MAX_PROJECTED_MESSAGES);
  const tool_results = collect_tool_results(projected_messages);
  const live_round_id_set = new Set(live_round_ids);
  const relevant_pending_permissions = filter_pending_permissions_for_stage(
    pending_permissions,
    session_key,
    agent_id,
    projected_messages,
  );
  const pending_permission_matches = match_pending_permissions_to_tool_uses(
    relevant_pending_permissions,
    collect_unresolved_tool_use_candidates(projected_messages),
  );
  const agent_workspace_events = agent_id
    ? workspace_events.filter((event) => event.agent_id === agent_id)
    : workspace_events;
  const events: NexusOperationEvent[] = [];

  for (const message of projected_messages) {
    if (message.role !== "assistant") {
      continue;
    }

    for (const block of message.content) {
      if (block.type === "tool_use") {
        events.push(project_tool_use({
          block,
          message,
          session_key,
          result: tool_results.get(block.id),
          pending_permission: pending_permission_matches.matched_permissions_by_tool_use_id.get(block.id) ?? null,
          is_live_round: live_round_id_set.has(message.round_id),
        }));
        continue;
      }

      if (block.type === "task_progress") {
        events.push({
          id: `${message.message_id}:task-progress:${block.task_id}`,
          session_key: message.session_key,
          round_id: message.round_id,
          agent_id: message.agent_id,
          message_id: message.message_id,
          tool_use_id: block.tool_use_id ?? null,
          tool_name: block.last_tool_name ?? "Task",
          kind: "task_progress",
          surface: "task",
          phase: live_round_id_set.has(message.round_id) ? "running" : "done",
          title: block.description || "子任务进度",
          target: block.last_tool_name ?? block.task_id,
          summary: summarize_projected_value(block.usage),
          input_preview: {
            task_id: block.task_id,
            last_tool_name: block.last_tool_name ?? null,
          },
          evidence: [
            { type: "task", label: "task", value: block.task_id },
          ],
          updated_at: message.timestamp,
        });
        continue;
      }

      if (block.type === "system_event") {
        const system_event = project_system_event({
          block,
          message,
          session_key,
          is_live_round: live_round_id_set.has(message.round_id),
        });
        if (system_event) {
          events.push(system_event);
        }
      }
    }

    const summary_event = project_result_summary_event({
      message,
      projected_messages,
    });
    if (summary_event) {
      events.push(summary_event);
    }
  }

  for (const permission of pending_permission_matches.unmatched_permissions) {
    if (events.some((event) => event.phase === "waiting" && event.tool_name === permission.tool_name)) {
      continue;
    }

    events.push(project_unmatched_permission(permission, session_key, agent_id));
  }

  for (const live_round_id of live_round_ids) {
    if (events.some((event) => event.round_id === live_round_id)) {
      continue;
    }

    const placeholder = project_live_round_placeholder({
      live_round_id,
      session_key,
      agent_id,
      messages: projected_messages,
    });
    if (placeholder) {
      events.push(placeholder);
    }
  }

  const relevant_workspace_events = filter_workspace_events_for_stage(
    agent_workspace_events,
    session_key,
    events,
  );

  for (const workspace_event of relevant_workspace_events) {
    events.push(project_workspace_event(
      workspace_event,
      session_key,
      resolve_workspace_event_round_id(workspace_event, events),
    ));
  }

  const sorted_events = events
    .sort((left, right) => (left.updated_at || 0) - (right.updated_at || 0))
    .slice(-MAX_EVENTS);
  const runtime_events = sort_operation_runtime_events([
    ...build_operation_message_runtime_events({
      agent_id,
      live_round_ids: live_round_id_set,
      messages: projected_messages,
      pending_permission_matches,
      session_key,
      tool_results,
    }),
    ...relevant_workspace_events.map((workspace_event) => build_workspace_runtime_event({
      round_id: resolve_workspace_event_round_id(workspace_event, events),
      session_key,
      workspace_event,
    })),
  ]);
  const active_event = pick_operation_active_event(sorted_events);
  const recent_evidence = collect_recent_operation_evidence(sorted_events, MAX_EVIDENCE);

  return {
    key,
    session_key,
    active_event,
    events: sorted_events,
    runtime_events,
    recent_evidence,
    workspace_events: relevant_workspace_events.slice(0, 8),
    updated_at: Date.now(),
  };
}

function project_system_event({
  block,
  message,
  session_key,
  is_live_round,
}: {
  block: SystemEventContent;
  message: Extract<Message, { role: "assistant" }>;
  session_key: string | null;
  is_live_round: boolean;
}): NexusOperationEvent | null {
  if (!is_live_round) {
    return null;
  }

  const subtype = block.subtype ?? "status";
  if (
    subtype !== "api_retry" &&
    subtype !== "status" &&
    subtype !== "progress" &&
    subtype !== "requesting"
  ) {
    return null;
  }

  const is_api_retry = subtype === "api_retry";
  const title = is_api_retry
    ? block.label || "API 正在重试"
    : block.label || "桌面待命";
  const target = is_api_retry
    ? block.content || "模型请求暂未成功，正在重试"
    : block.content || "等待第一个应用窗口";

  return {
    id: `${message.message_id}:system:${subtype}:${block.timestamp}`,
    session_key: session_key ?? message.session_key,
    round_id: message.round_id,
    agent_id: message.agent_id,
    message_id: message.message_id,
    kind: "unknown",
    surface: "conversation",
    phase: "running",
    title,
    target,
    summary: target,
    evidence: [
      { type: "status", label: subtype, value: title },
    ],
    started_at: message.timestamp,
    updated_at: block.timestamp || message.timestamp,
  };
}

function project_live_round_placeholder({
  live_round_id,
  session_key,
  agent_id,
  messages,
}: {
  live_round_id: string;
  session_key: string | null;
  agent_id?: string | null;
  messages: Message[];
}): NexusOperationEvent | null {
  const related_message = [...messages].reverse().find((message) => message.round_id === live_round_id);
  if (!related_message) {
    return null;
  }

  const user_prompt = related_message.role === "user"
    ? related_message.content
    : find_round_user_prompt(messages, live_round_id);
  const resolved_agent_id = agent_id
    ?? ("agent_id" in related_message ? related_message.agent_id : null);
  if (!resolved_agent_id) {
    return null;
  }

  return {
    id: `live-round:${live_round_id}`,
    session_key: session_key ?? related_message.session_key,
    round_id: live_round_id,
    agent_id: resolved_agent_id,
    message_id: related_message.message_id,
    kind: "unknown",
    surface: "conversation",
    phase: "running",
    title: "桌面待命",
    target: "等待第一个应用窗口",
    summary: user_prompt || "Nexus 桌面已打开，还没有进入具体工具。",
    input_preview: user_prompt ? { prompt: user_prompt } : null,
    evidence: [
      { type: "status", label: "round", value: "running" },
    ],
    started_at: related_message.timestamp,
    updated_at: Date.now(),
  };
}

function find_round_user_prompt(messages: Message[], round_id: string): string | null {
  const user_message = [...messages].reverse().find((message) => (
    message.round_id === round_id &&
    message.role === "user"
  ));
  return user_message?.role === "user" ? user_message.content : null;
}

function collect_tool_results(messages: Message[]): Map<string, ToolResultContent> {
  const results = new Map<string, ToolResultContent>();

  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }
    for (const block of message.content) {
      if (block.type === "tool_result") {
        results.set(block.tool_use_id, block);
      }
    }
  }

  return results;
}

function project_tool_use({
  block,
  message,
  session_key,
  result,
  pending_permission,
  is_live_round,
}: {
  block: ToolUseContent;
  message: Extract<Message, { role: "assistant" }>;
  session_key: string | null;
  result?: ToolResultContent;
  pending_permission?: PendingPermission | null;
  is_live_round: boolean;
}): NexusOperationEvent {
  const projection = resolve_operation_tool_profile(block.name);
  const input_preview = redact_projected_value(as_record(block.input)) as Record<string, unknown>;
  const target = extract_target(input_preview, projection.target_keys) ?? block.name;
  const phase = resolve_tool_phase(result, pending_permission, is_live_round, message.is_complete);
  const evidence = build_tool_evidence(block.name, target, result, pending_permission);

  return {
    id: `${message.message_id}:${block.id}`,
    session_key: session_key ?? message.session_key,
    round_id: message.round_id,
    agent_id: message.agent_id,
    message_id: message.message_id,
    tool_use_id: block.id,
    tool_name: block.name,
    kind: projection.kind,
    surface: projection.surface,
    phase,
    title: projection.title,
    target,
    summary: pending_permission?.summary ?? summarize_result(result),
    input_preview,
    result_preview: build_tool_result_preview(result, projection.kind),
    evidence,
    permission_request_id: pending_permission?.request_id ?? null,
    permission_interaction_mode: pending_permission?.interaction_mode ?? null,
    started_at: message.timestamp,
    updated_at: message.timestamp,
    ended_at: result ? message.timestamp : null,
  };
}

function resolve_tool_phase(
  result: ToolResultContent | undefined,
  pending_permission: PendingPermission | null | undefined,
  is_live_round: boolean,
  is_complete: boolean | undefined,
): OperationPhase {
  if (pending_permission) {
    return "waiting";
  }
  if (result?.is_error) {
    return "error";
  }
  if (result) {
    return "done";
  }
  if (is_live_round || !is_complete) {
    return "running";
  }
  return "done";
}

function build_tool_evidence(
  tool_name: string,
  target: string | null,
  result: ToolResultContent | undefined,
  pending_permission: PendingPermission | null | undefined,
): OperationEvidence[] {
  const profile = resolve_operation_tool_profile(tool_name);
  const evidence: OperationEvidence[] = [];
  if (target) {
    evidence.push({ type: profile.evidence_type, label: profile.action_label, value: target });
  }
  if (pending_permission) {
    evidence.push({
      type: "permission",
      label: pending_permission.risk_label || "waiting",
      value: pending_permission.summary || pending_permission.tool_name,
    });
  }
  if (result?.is_error) {
    evidence.push({ type: "error", label: "error", value: summarize_projected_value(result.content) });
  } else if (result) {
    evidence.push({ type: "status", label: "result", value: summarize_projected_value(result.content) });
  }
  return evidence;
}

function build_tool_result_preview(
  result: ToolResultContent | undefined,
  kind: NexusOperationEvent["kind"],
): unknown {
  if (!result) {
    return null;
  }

  const redacted_content = redact_projected_value(result.content);
  if (kind === "command_run" || kind === "command_stop") {
    return {
      content: redacted_content,
      error_code: result.error_code ?? null,
      is_error: Boolean(result.is_error),
    };
  }
  return redacted_content;
}

function project_unmatched_permission(
  permission: PendingPermission,
  session_key: string | null,
  agent_id?: string | null,
): NexusOperationEvent {
  const profile = resolve_operation_tool_profile(permission.tool_name);
  const target = extract_target(
    redact_projected_value(permission.tool_input) as Record<string, unknown>,
    profile.target_keys,
  ) ?? permission.tool_name;
  return {
    id: `permission:${permission.request_id}`,
    session_key: session_key ?? permission.session_key ?? "",
    round_id: permission.caused_by ?? permission.request_id,
    agent_id: permission.agent_id ?? agent_id ?? "",
    message_id: permission.message_id ?? null,
    tool_name: permission.tool_name,
    kind: "human_gate",
    surface: "conversation",
    phase: "waiting",
    title: permission.interaction_mode === "question" ? "等待用户回答" : "等待权限确认",
    target,
    summary: permission.summary ?? permission.risk_label ?? null,
    input_preview: redact_projected_value(permission.tool_input) as Record<string, unknown>,
    evidence: [
      { type: "permission", label: permission.risk_label || "waiting", value: permission.summary ?? permission.tool_name },
    ],
    permission_request_id: permission.request_id,
    permission_interaction_mode: permission.interaction_mode ?? (
      permission.tool_name === "AskUserQuestion" ? "question" : "permission"
    ),
    updated_at: Date.now(),
  };
}

function project_workspace_event(
  event: WorkspaceActivityItem,
  session_key: string | null,
  round_id: string,
): NexusOperationEvent {
  const is_deleted = event.status === "deleted";
  const is_done = event.status === "updated" || is_deleted;

  return {
    id: `workspace:${event.id}`,
    session_key: session_key ?? event.session_key ?? "",
    round_id,
    agent_id: event.agent_id,
    tool_use_id: event.tool_use_id ?? null,
    tool_name: "workspace_event",
    kind: is_deleted ? "workspace_edit" : "workspace_edit",
    surface: "editor",
    phase: is_done ? "done" : "running",
    title: is_deleted ? "删除工作区文件" : "写入工作区文件",
    target: event.path,
    summary: event.diff_stats
      ? `+${event.diff_stats.additions} -${event.diff_stats.deletions}`
      : null,
    result_preview: event.live_content ? truncate_projected_text(event.live_content, OPERATION_MAX_TEXT_PREVIEW) : null,
    evidence: [
      { type: "file", label: event.status, value: event.path },
      ...(event.diff_stats ? [{
        type: "diff" as const,
        label: "diff",
        value: `+${event.diff_stats.additions} -${event.diff_stats.deletions}`,
      }] : []),
    ],
    updated_at: event.updated_at,
  };
}

function extract_target(input: Record<string, unknown>, keys: readonly string[]): string | null {
  const primary = extract_operation_input_value(input, keys);
  if (primary?.value) {
    return truncate_projected_text(primary.value, 96);
  }
  const fallback = extract_operation_input_value(input, DEFAULT_TARGET_KEYS);
  return fallback?.value ? truncate_projected_text(fallback.value, 96) : null;
}

function summarize_result(result?: ToolResultContent): string | null {
  if (!result) {
    return null;
  }
  return summarize_projected_value(result.content);
}

function as_record(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}
