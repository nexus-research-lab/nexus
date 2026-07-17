/**
 * INPUT: One operation event, its snapshot, and semantic window configuration.
 * OUTPUT: A normalized StageWindowState with truthful title, target, and payload defaults.
 * POS: Pure window-construction boundary used by operation-scene-planner.
 */
import type {
  StageWindowKind,
  StageWindowLayout,
  StageWindowPayload,
  StageWindowPhase,
  StageWindowState,
} from "./operation-desktop-types";
import {
  fallbackStageEventObjectLabel,
  fallbackStageEventTargetLabel,
  isLowSignalStageLabel,
} from "./operation-stage-labels";
import type { NexusOperationEvent, NexusOperationSnapshot } from "./operation-types";

export function buildOperationStageWindow(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
  config: {
    id: string;
    session_id?: string;
    kind: StageWindowKind;
    title: string;
    layout: StageWindowLayout;
    phase: StageWindowPhase;
    z: number;
    payload?: Partial<StageWindowPayload>;
  },
): StageWindowState {
  return {
    id: config.session_id ?? `${event.id}:${config.id}`,
    kind: config.kind,
    title: normalize_stage_window_title(event, config.title),
    subtitle: normalize_stage_window_subtitle(event),
    target: normalize_stage_window_target(event, config.payload?.target),
    phase: config.phase,
    z: config.z,
    layout: config.layout,
    payload: {
      event,
      snapshot,
      summary: event.summary,
      target: event.target,
      ...config.payload,
    },
  };
}

function normalize_stage_window_title(event: NexusOperationEvent, title: string): string {
  return isLowSignalStageLabel(title) ? fallbackStageEventObjectLabel(event) : title;
}

function normalize_stage_window_subtitle(event: NexusOperationEvent): string | null {
  return !event.summary || isLowSignalStageLabel(event.summary) ? null : event.summary;
}

function normalize_stage_window_target(
  event: NexusOperationEvent,
  target: string | null | undefined,
): string | null {
  const candidate = target ?? event.target;
  if (!candidate) {
    return null;
  }
  return isLowSignalStageLabel(candidate) ? fallbackStageEventTargetLabel(event) : candidate;
}
