/**
 * INPUT: Current Operation Stage event stream containing live and settled permission checkpoints.
 * OUTPUT: The one actionable permission notification and its truthful user-facing command summary.
 * POS: Pure permission notification model shared by the Stage view and regression tests.
 */
import type { NexusOperationEvent } from "../operation-types";

export function findWaitingPermissionEvent(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
): NexusOperationEvent | null {
  const resolved_request_ids = new Set(events.flatMap((item) => (
    item.permission_request_id && item.permission_decision
      ? [item.permission_request_id]
      : []
  )));
  const candidates = [event, ...events]
    .filter((item, index, items) => (
      item.permission_request_id
      && item.phase === "waiting"
      && !resolved_request_ids.has(item.permission_request_id)
      && items.findIndex((candidate) => candidate.id === item.id) === index
    ))
    .sort((left, right) => right.updated_at - left.updated_at);

  return candidates.find((candidate) => !isPermissionSuperseded(candidate, events)) ?? null;
}

export function readPermissionCommand(event: NexusOperationEvent): string | null {
  const command = event.input_preview?.command;
  if (typeof command === "string" && command.trim()) {
    return command.trim();
  }
  return event.target?.trim() || null;
}

export function readPermissionSummary(
  event: NexusOperationEvent,
  command: string | null,
): string {
  if (event.permission_interaction_mode === "question") {
    return event.summary ?? event.target ?? "等待你补充信息后继续。";
  }
  const summary = event.summary?.trim();
  if (summary && summary !== command && summary !== event.target) {
    return summary;
  }
  if (event.tool_name === "Bash" || command) {
    return "允许 Nexus 运行这条终端命令。";
  }
  return event.target ?? event.tool_name ?? "允许 Nexus 执行这项操作。";
}

function isPermissionSuperseded(
  candidate: NexusOperationEvent,
  events: NexusOperationEvent[],
): boolean {
  return events.some((item) => (
    item.round_id === candidate.round_id
    && item.updated_at > candidate.updated_at
    && item.phase !== "waiting"
    && (
      item.permission_request_id === candidate.permission_request_id
      || Boolean(candidate.tool_use_id && item.tool_use_id === candidate.tool_use_id)
    )
  ));
}
