import type { WorkspaceActivityItem } from "@/types/app/workspace-live";

import type {
  NexusOperationEvent,
  OperationEvidence,
  OperationPhase,
} from "./operation-types";

export function filterWorkspaceEventsForStage(
  workspace_events: WorkspaceActivityItem[],
  session_key: string | null,
  projected_events: NexusOperationEvent[],
): WorkspaceActivityItem[] {
  if (!session_key) {
    return workspace_events;
  }

  const tool_use_ids = new Set(
    projected_events
      .map((event) => event.tool_use_id)
      .filter((tool_use_id): tool_use_id is string => Boolean(tool_use_id)),
  );

  return workspace_events.filter((event) => {
    if (event.session_key === session_key) {
      return true;
    }
    return Boolean(event.tool_use_id && tool_use_ids.has(event.tool_use_id));
  });
}

export function pickOperationActiveEvent(events: NexusOperationEvent[]): NexusOperationEvent | null {
  if (!events.length) {
    return null;
  }

  const latest_event = events.reduce((latest, item) => (
    (item.updated_at || 0) >= (latest.updated_at || 0) ? item : latest
  ), events[0]);
  const active_round_id = latest_event.round_id;
  const round_events = events.filter((item) => item.round_id === active_round_id);
  const summary_event = [...round_events].reverse().find((item) => item.kind === "round_summary");
  if (summary_event && (
    summary_event.phase === "done" ||
    summary_event.phase === "error" ||
    summary_event.phase === "cancelled"
  )) {
    return summary_event;
  }

  const priority = ["waiting", "running", "error"] satisfies OperationPhase[];

  for (const phase of priority) {
    const event = [...round_events].reverse().find((item) => item.phase === phase);
    if (event) {
      return event;
    }
  }

  if (summary_event) {
    return summary_event;
  }

  return round_events.at(-1) ?? latest_event;
}

export function resolveWorkspaceEventRoundId(
  event: WorkspaceActivityItem,
  projected_events: NexusOperationEvent[],
): string {
  if (event.tool_use_id) {
    const matched_tool_event = [...projected_events].reverse().find((item) => (
      item.tool_use_id === event.tool_use_id &&
      item.agent_id === event.agent_id
    ));
    if (matched_tool_event) {
      return matched_tool_event.round_id;
    }
    return event.tool_use_id;
  }
  return event.session_key ?? event.id;
}

export function collectRecentOperationEvidence(
  events: NexusOperationEvent[],
  max_evidence: number,
): OperationEvidence[] {
  return events
    .flatMap((event) => event.evidence ?? [])
    .slice(-max_evidence);
}
