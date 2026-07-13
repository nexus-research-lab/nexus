import type { StageNarrativePhase } from "./operation-stage-model";
import type { NexusOperationEvent } from "../operation-types";

export function countDesktopRevealEvents(events: NexusOperationEvent[]): number {
  return events.filter((event) => (
    Boolean(event.tool_use_id) ||
    event.surface !== "conversation"
  )).length;
}

export function initialRevealedWindowCount({
  minimum_count,
  phase,
  window_count,
}: {
  minimum_count: number;
  phase: StageNarrativePhase;
  window_count: number;
}): number {
  if (window_count <= 0) {
    return 0;
  }
  if (phase === "completed" || phase === "settling") {
    return window_count;
  }
  return Math.min(window_count, Math.max(1, minimum_count));
}
