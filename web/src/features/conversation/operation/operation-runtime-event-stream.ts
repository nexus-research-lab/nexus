import type { WorkspaceActivityItem } from "@/types/app/workspace-live";
import type {
  SystemEventContent,
  TaskProgressContent,
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message/content";
import type { Message } from "@/types/conversation/message/entity";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import type { PendingPermissionMatchResult } from "./operation-pending-permissions";
import type { OperationRuntimeEvent } from "./operation-runtime-types";
import {
  redactProjectedValue,
  redactProjectedTerminalValue,
  summarizeProjectedValue,
} from "./operation-projection-preview";
import { projectResultSummaryEvent } from "./operation-summary-events";

const MAX_RUNTIME_EVENTS = 80;

export interface BuildOperationMessageRuntimeEventsParams {
  agent_id?: string | null;
  live_round_ids: Set<string>;
  messages: Message[];
  pending_permission_matches: PendingPermissionMatchResult;
  session_key: string | null;
  tool_results: Map<string, ToolResultContent>;
}

export function buildOperationMessageRuntimeEvents({
  agent_id,
  live_round_ids,
  messages,
  pending_permission_matches,
  session_key,
  tool_results,
}: BuildOperationMessageRuntimeEventsParams): OperationRuntimeEvent[] {
  const runtime_events: OperationRuntimeEvent[] = [];
  const toolProgressByUseId = collectToolProgress(messages);

  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }

    for (const block of message.content) {
      if (block.type === "tool_use") {
        runtime_events.push(...build_tool_runtime_events({
          block,
          message,
          pending_permission: pending_permission_matches.matched_permissions_by_tool_use_id.get(block.id) ?? null,
          progress: toolProgressByUseId.get(block.id),
          result: tool_results.get(block.id),
          session_key,
          is_live_round: live_round_ids.has(message.round_id),
        }));
        continue;
      }

      if (block.type === "task_progress") {
        if (isTerminalToolProgress(block)) {
          continue;
        }
        runtime_events.push({
          id: `runtime:${message.message_id}:task-progress:${block.task_id}:delta`,
          event_type: "tool_delta",
          session_key: session_key ?? message.session_key,
          round_id: message.round_id,
          agent_id: message.agent_id,
          message_id: message.message_id,
          tool_use_id: block.tool_use_id ?? null,
          tool_name: block.last_tool_name ?? "TaskOutput",
          phase: live_round_ids.has(message.round_id) ? "running" : "done",
          timestamp: message.timestamp,
          input: {
            task_id: block.task_id,
            last_tool_name: block.last_tool_name ?? null,
          },
          delta: redactProjectedValue(block),
        });
        continue;
      }

      if (block.type === "system_event") {
        const runtime_event = build_system_runtime_event({
          block,
          message,
          session_key,
        });
        if (runtime_event) {
          runtime_events.push(runtime_event);
        }
      }
    }

    const summary_event = projectResultSummaryEvent({
      message,
      projected_messages: messages,
    });
    if (summary_event) {
      runtime_events.push({
        id: `runtime:${summary_event.id}:handoff`,
        event_type: "round_handoff",
        session_key: summary_event.session_key,
        round_id: summary_event.round_id,
        agent_id: summary_event.agent_id,
        message_id: summary_event.message_id,
        tool_use_id: summary_event.tool_use_id ?? null,
        tool_name: summary_event.tool_name ?? "RoundSummary",
        phase: summary_event.phase,
        timestamp: summary_event.updated_at,
        input: summary_event.input_preview ?? null,
        result: summary_event.result_preview ?? summary_event.summary ?? null,
        artifact: {
          kind: "handoff",
          preview: summary_event.result_preview ?? summary_event.summary ?? null,
        },
        source_event_id: summary_event.id,
      });
    }
  }

  for (const permission of pending_permission_matches.unmatched_permissions) {
    runtime_events.push(build_permission_runtime_event({
      agent_id,
      permission,
      session_key,
    }));
  }

  return sortOperationRuntimeEvents(runtime_events);
}

export function buildWorkspaceRuntimeEvent({
  round_id,
  session_key,
  workspace_event,
}: {
  round_id: string;
  session_key: string | null;
  workspace_event: WorkspaceActivityItem;
}): OperationRuntimeEvent {
  const is_deleted = workspace_event.status === "deleted";
  const is_done = workspace_event.status === "updated" || is_deleted;
  const artifact_kind = /\.x?html?$/i.test(workspace_event.path)
    ? "html"
    : "workspace_file";

  return {
    id: `runtime:workspace:${workspace_event.id}`,
    event_type: "artifact_update",
    session_key: session_key ?? workspace_event.session_key ?? null,
    round_id,
    agent_id: workspace_event.agent_id,
    tool_use_id: workspace_event.tool_use_id ?? null,
    tool_name: "workspace_event",
    phase: is_done ? "done" : "running",
    timestamp: workspace_event.updated_at,
    input: {
      path: workspace_event.path,
      status: workspace_event.status,
    },
    delta: {
      event_type: workspace_event.event_type,
      status: workspace_event.status,
    },
    artifact: {
      kind: artifact_kind,
      path: workspace_event.path,
      status: workspace_event.status,
      live_content: workspace_event.live_content ?? null,
      diff_stats: workspace_event.diff_stats ?? null,
    },
  };
}

export function sortOperationRuntimeEvents(
  runtime_events: OperationRuntimeEvent[],
): OperationRuntimeEvent[] {
  return runtime_events
    .sort((left, right) => left.timestamp - right.timestamp)
    .slice(-MAX_RUNTIME_EVENTS);
}

function build_tool_runtime_events({
  block,
  message,
  pending_permission,
  progress,
  result,
  session_key,
  is_live_round,
}: {
  block: ToolUseContent;
  message: Extract<Message, { role: "assistant" }>;
  pending_permission: PendingPermission | null;
  progress?: TaskProgressContent;
  result?: ToolResultContent;
  session_key: string | null;
  is_live_round: boolean;
}): OperationRuntimeEvent[] {
  const input = redactProjectedValue(as_record(block.input)) as Record<string, unknown>;
  const durationMs = readProgressDurationMs(progress);
  const runtime_events: OperationRuntimeEvent[] = [{
    id: `runtime:${message.message_id}:${block.id}:start`,
    event_type: "tool_start",
    session_key: session_key ?? message.session_key,
    round_id: message.round_id,
    agent_id: message.agent_id,
    message_id: message.message_id,
    tool_use_id: block.id,
    tool_name: block.name,
    phase: pending_permission ? "waiting" : "running",
    timestamp: message.timestamp,
    duration_ms: durationMs,
    input,
  }];

  if (pending_permission) {
    runtime_events.push(build_permission_runtime_event({
      agent_id: message.agent_id,
      permission: pending_permission,
      round_id: message.round_id,
      session_key: session_key ?? message.session_key,
      timestamp: message.timestamp + 1,
      tool_use_id: block.id,
    }));
  }

  if (!result && !pending_permission && (is_live_round || !message.is_complete)) {
    runtime_events.push({
      id: `runtime:${message.message_id}:${block.id}:running`,
      event_type: "tool_delta",
      session_key: session_key ?? message.session_key,
      round_id: message.round_id,
      agent_id: message.agent_id,
      message_id: message.message_id,
      tool_use_id: block.id,
      tool_name: block.name,
      phase: "running",
      timestamp: durationMs == null ? message.timestamp : message.timestamp + durationMs,
      duration_ms: durationMs,
      input,
      delta: {
        status: "running",
        ...(durationMs == null ? {} : { duration_ms: durationMs }),
      },
    });
  }

  if (result) {
    runtime_events.push({
      id: `runtime:${message.message_id}:${block.id}:end`,
      event_type: "tool_end",
      session_key: session_key ?? message.session_key,
      round_id: message.round_id,
      agent_id: message.agent_id,
      message_id: message.message_id,
      tool_use_id: block.id,
      tool_name: block.name,
      phase: result.is_error ? "error" : "done",
      timestamp: durationMs == null ? message.timestamp : message.timestamp + durationMs,
      duration_ms: durationMs,
      input,
      result: build_runtime_result(result, block.name),
    });
  }

  return runtime_events;
}

function build_system_runtime_event({
  block,
  message,
  session_key,
}: {
  block: SystemEventContent;
  message: Extract<Message, { role: "assistant" }>;
  session_key: string | null;
}): OperationRuntimeEvent | null {
  const subtype = block.subtype ?? "status";
  if (
    subtype !== "api_retry" &&
    subtype !== "status" &&
    subtype !== "progress" &&
    subtype !== "requesting"
  ) {
    return null;
  }

  return {
    id: `runtime:${message.message_id}:system:${subtype}:${block.timestamp}`,
    event_type: "tool_delta",
    session_key: session_key ?? message.session_key,
    round_id: message.round_id,
    agent_id: message.agent_id,
    message_id: message.message_id,
    tool_name: "system",
    phase: "running",
    timestamp: block.timestamp || message.timestamp,
    delta: {
      subtype,
      label: block.label ?? null,
      content: block.content ?? null,
    },
  };
}

function build_permission_runtime_event({
  agent_id,
  permission,
  round_id,
  session_key,
  timestamp,
  tool_use_id,
}: {
  agent_id?: string | null;
  permission: PendingPermission;
  round_id?: string | null;
  session_key: string | null;
  timestamp?: number;
  tool_use_id?: string | null;
}): OperationRuntimeEvent {
  return {
    id: `runtime:permission:${permission.request_id}`,
    event_type: "permission_request",
    session_key: session_key ?? permission.session_key ?? null,
    round_id: round_id ?? permission.round_id ?? permission.agent_round_id ?? permission.request_id,
    agent_id: permission.agent_id ?? agent_id ?? "",
    message_id: permission.message_id ?? null,
    tool_use_id: tool_use_id ?? null,
    tool_name: permission.tool_name,
    phase: "waiting",
    timestamp: timestamp ?? resolve_permission_timestamp(permission),
    input: redactProjectedValue(permission.tool_input) as Record<string, unknown>,
    delta: {
      summary: permission.summary ?? null,
      risk_label: permission.risk_label ?? null,
      suggestions: permission.suggestions ?? [],
    },
    permission_request_id: permission.request_id,
    permission_interaction_mode: permission.interaction_mode ?? (
      permission.tool_name === "AskUserQuestion" ? "question" : "permission"
    ),
    permission_risk_level: permission.risk_level ?? null,
  };
}

function resolve_permission_timestamp(permission: PendingPermission): number {
  const expires_at_ms = permission.expires_at ? Date.parse(permission.expires_at) : NaN;
  return Number.isFinite(expires_at_ms) ? expires_at_ms : 0;
}

function collectToolProgress(messages: Message[]): Map<string, TaskProgressContent> {
  const progressByToolUseId = new Map<string, TaskProgressContent>();
  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }
    for (const block of message.content) {
      if (block.type === "task_progress" && block.tool_use_id) {
        progressByToolUseId.set(block.tool_use_id, block);
      }
    }
  }
  return progressByToolUseId;
}

function isTerminalToolProgress(progress: TaskProgressContent): boolean {
  return progress.last_tool_name === "Bash" || progress.last_tool_name === "KillShell";
}

function readProgressDurationMs(progress?: TaskProgressContent): number | null {
  const raw = progress?.usage?.duration_ms;
  if (typeof raw === "number" && Number.isFinite(raw)) {
    return Math.max(0, raw);
  }
  if (typeof raw === "string" && /^\d+(?:\.\d+)?$/.test(raw.trim())) {
    return Number(raw);
  }
  return null;
}

function build_runtime_result(result: ToolResultContent, toolName: string): unknown {
  const isTerminal = toolName === "Bash" || toolName === "KillShell";
  const redacted_content = isTerminal
    ? redactProjectedTerminalValue(result.content)
    : redactProjectedValue(result.content);
  if (isTerminal) {
    return {
      content: redacted_content,
      error_code: result.error_code ?? null,
      is_error: Boolean(result.is_error),
    };
  }
  if (result.error_code || result.is_error) {
    return {
      content: redacted_content,
      error_code: result.error_code ?? null,
      is_error: Boolean(result.is_error),
    };
  }
  if (typeof redacted_content === "string") {
    return redacted_content;
  }
  return {
    content: redacted_content,
    is_error: false,
    summary: summarizeProjectedValue(result.content),
  };
}

function as_record(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}
