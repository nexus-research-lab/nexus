/**
 * INPUT: Restored Operation Stage history and the latest live conversation projection.
 * OUTPUT: A merged desktop snapshot that preserves review context without reviving transient permission prompts.
 * POS: Operation Stage snapshot reconciliation boundary between persisted history and live runtime state.
 */
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";

const MAX_MERGED_EVENTS = 24;
const MAX_MERGED_WORKSPACE_EVENTS = 8;
const MAX_MERGED_EVIDENCE = 8;

export function mergeOperationStageSnapshotsForRestore(
  restored: NexusOperationSnapshot | null | undefined,
  projected: NexusOperationSnapshot,
): NexusOperationSnapshot {
  if (!restored || restored.key !== projected.key) {
    return projected;
  }
  if (
    restored.session_key
    && projected.session_key
    && restored.session_key !== projected.session_key
  ) {
    return projected;
  }

  const active_round_id = projected.active_event?.round_id
    ?? projected.events.at(-1)?.round_id
    ?? null;
  if (!active_round_id) {
    return projected;
  }

  const active_permission_request_ids = collectActivePermissionRequestIds(projected);
  const sanitized_restored = sanitizePermissionArtifacts(
    restored,
    active_permission_request_ids,
  );
  const terminal_round_summary = [...projected.events]
    .reverse()
    .find((event) => (
      event.round_id === active_round_id
      && event.kind === "round_summary"
      && (event.phase === "done" || event.phase === "error" || event.phase === "cancelled")
    )) ?? null;
  const restored_round_events = sanitized_restored.events
    .filter((event) => event.round_id === active_round_id)
    .map((event) => settleStaleLiveEventForRoundSummary(event, terminal_round_summary));
  if (!restored_round_events.length) {
    return projected;
  }

  const projected_event_ids = new Set(projected.events.map((event) => event.id));
  const merged_events = [
    ...restored_round_events.filter((event) => !projected_event_ids.has(event.id)),
    ...projected.events,
  ]
    .sort((left, right) => left.updated_at - right.updated_at)
    .slice(-MAX_MERGED_EVENTS);

  return {
    ...projected,
    active_event: projected.active_event ?? merged_events.at(-1) ?? null,
    events: merged_events,
    runtime_events: mergeRuntimeEventsForRound(
      sanitized_restored.runtime_events ?? [],
      projected.runtime_events ?? [],
      active_round_id,
    ),
    recent_evidence: mergeOperationEvidence(
      sanitized_restored.recent_evidence,
      projected.recent_evidence,
    ),
    workspace_events: mergeWorkspaceEventsForRound(
      sanitized_restored.workspace_events,
      projected.workspace_events,
      merged_events,
    ),
    updated_at: Math.max(restored.updated_at, projected.updated_at),
  };
}

export function sanitizeOperationStageSnapshotForRestore(
  snapshot: NexusOperationSnapshot,
): NexusOperationSnapshot {
  return sanitizePermissionArtifacts(snapshot, new Set());
}

function sanitizePermissionArtifacts(
  snapshot: NexusOperationSnapshot,
  active_request_ids: ReadonlySet<string>,
): NexusOperationSnapshot {
  const stale_tool_use_ids = collectStalePermissionToolUseIds(
    snapshot,
    active_request_ids,
  );
  const events = snapshot.events.filter((event) => (
    shouldKeepPermissionEvent(event, active_request_ids)
  ));
  const runtime_events = (snapshot.runtime_events ?? []).filter((event) => {
    if (
      event.event_type === "permission_request"
      && event.permission_request_id
      && !active_request_ids.has(event.permission_request_id)
    ) {
      return false;
    }
    return !(
      event.phase === "waiting"
      && event.tool_use_id
      && stale_tool_use_ids.has(event.tool_use_id)
    );
  });
  const active_event = snapshot.active_event
    ? events.find((event) => event.id === snapshot.active_event?.id) ?? events.at(-1) ?? null
    : events.at(-1) ?? null;

  return {
    ...snapshot,
    active_event,
    events,
    runtime_events,
    recent_evidence: snapshot.recent_evidence.filter((item) => item.type !== "permission"),
  };
}

function collectActivePermissionRequestIds(
  snapshot: NexusOperationSnapshot,
): Set<string> {
  const request_ids = new Set<string>();
  for (const event of snapshot.events) {
    if (event.phase === "waiting" && event.permission_request_id) {
      request_ids.add(event.permission_request_id);
    }
  }
  for (const event of snapshot.runtime_events ?? []) {
    if (
      event.event_type === "permission_request"
      && event.phase === "waiting"
      && event.permission_request_id
    ) {
      request_ids.add(event.permission_request_id);
    }
  }
  return request_ids;
}

function collectStalePermissionToolUseIds(
  snapshot: NexusOperationSnapshot,
  active_request_ids: ReadonlySet<string>,
): Set<string> {
  const tool_use_ids = new Set<string>();
  for (const event of snapshot.events) {
    if (
      event.tool_use_id
      && event.permission_request_id
      && !event.permission_decision
      && !active_request_ids.has(event.permission_request_id)
    ) {
      tool_use_ids.add(event.tool_use_id);
    }
  }
  for (const event of snapshot.runtime_events ?? []) {
    if (
      event.tool_use_id
      && event.event_type === "permission_request"
      && event.permission_request_id
      && !active_request_ids.has(event.permission_request_id)
    ) {
      tool_use_ids.add(event.tool_use_id);
    }
  }
  return tool_use_ids;
}

function shouldKeepPermissionEvent(
  event: NexusOperationEvent,
  active_request_ids: ReadonlySet<string>,
): boolean {
  if (!event.permission_request_id || event.permission_decision) {
    return true;
  }
  return active_request_ids.has(event.permission_request_id);
}

function mergeRuntimeEventsForRound(
  restored: NexusOperationSnapshot["runtime_events"],
  projected: NexusOperationSnapshot["runtime_events"],
  round_id: string,
): NexusOperationSnapshot["runtime_events"] {
  const projected_ids = new Set(projected.map((event) => event.id));
  return [
    ...restored.filter((event) => event.round_id === round_id && !projected_ids.has(event.id)),
    ...projected,
  ]
    .sort((left, right) => left.timestamp - right.timestamp)
    .slice(-MAX_MERGED_EVENTS);
}

function settleStaleLiveEventForRoundSummary(
  event: NexusOperationEvent,
  summary: NexusOperationEvent | null,
): NexusOperationEvent {
  if (
    !summary
    || (event.phase !== "running" && event.phase !== "waiting" && event.phase !== "queued")
    || event.id === summary.id
  ) {
    return event;
  }

  const settled_phase = summary.phase === "cancelled"
    ? "cancelled"
    : summary.phase === "error"
      ? "error"
      : "done";
  return {
    ...event,
    phase: settled_phase,
    summary: event.summary ?? summary.summary,
    ended_at: summary.ended_at ?? summary.updated_at,
    updated_at: Math.max(event.updated_at, summary.updated_at),
    evidence: [
      ...(event.evidence ?? []),
      {
        type: summary.phase === "error" ? "error" : "status",
        label: summary.phase === "error" ? "round_error" : "round_settled",
        value: summary.summary ?? summary.title,
      },
    ],
  };
}

function mergeWorkspaceEventsForRound(
  restored: NexusOperationSnapshot["workspace_events"],
  projected: NexusOperationSnapshot["workspace_events"],
  events: NexusOperationSnapshot["events"],
): NexusOperationSnapshot["workspace_events"] {
  const round_tool_use_ids = new Set(
    events
      .map((event) => event.tool_use_id)
      .filter((tool_use_id): tool_use_id is string => Boolean(tool_use_id)),
  );
  const round_targets = new Set(
    events
      .map((event) => event.target)
      .filter((target): target is string => Boolean(target)),
  );
  const merged_by_id = new Map<string, NexusOperationSnapshot["workspace_events"][number]>();

  for (const item of restored) {
    if (
      (item.tool_use_id && round_tool_use_ids.has(item.tool_use_id))
      || round_targets.has(item.path)
    ) {
      merged_by_id.set(item.id, item);
    }
  }
  for (const item of projected) {
    merged_by_id.set(item.id, item);
  }

  return Array.from(merged_by_id.values())
    .sort((left, right) => right.updated_at - left.updated_at)
    .slice(0, MAX_MERGED_WORKSPACE_EVENTS);
}

function mergeOperationEvidence(
  restored: NexusOperationSnapshot["recent_evidence"],
  projected: NexusOperationSnapshot["recent_evidence"],
): NexusOperationSnapshot["recent_evidence"] {
  const merged = new Map<string, NexusOperationSnapshot["recent_evidence"][number]>();
  for (const item of [...restored, ...projected]) {
    merged.set(`${item.type}:${item.label}:${item.value ?? ""}`, item);
  }
  return Array.from(merged.values()).slice(-MAX_MERGED_EVIDENCE);
}
