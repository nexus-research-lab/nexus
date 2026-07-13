import type { CSSProperties } from "react";

import type { NexusOperationEvent } from "./operation-types";
import { SURFACE_META } from "./operation-stage-panel-style";
import type { SurfaceMeta } from "./operation-stage-panel-style";

export type StageTransitionIntent =
  | "browser"
  | "editor"
  | "permission"
  | "summary"
  | "task"
  | "terminal"
  | "workspace";

export function surfaceMetaForTransition(
  event: NexusOperationEvent,
  intent: StageTransitionIntent,
): SurfaceMeta {
  if (event.surface !== "fallback") {
    return SURFACE_META[event.surface];
  }
  if (intent === "browser") {
    return SURFACE_META.web;
  }
  if (intent === "terminal") {
    return SURFACE_META.terminal;
  }
  if (intent === "workspace") {
    return SURFACE_META.workspace;
  }
  if (intent === "editor") {
    return SURFACE_META.editor;
  }
  if (intent === "task") {
    return SURFACE_META.task;
  }
  if (intent === "permission") {
    return SURFACE_META.conversation;
  }
  return SURFACE_META.summary;
}

export function buildStageTransitionStyle(intent: StageTransitionIntent): CSSProperties {
  const map: Record<StageTransitionIntent, Record<string, string>> = {
    browser: {
      "--operation-scene-enter-x": "28px",
      "--operation-scene-enter-y": "4px",
    },
    editor: {
      "--operation-scene-enter-x": "0",
      "--operation-scene-enter-y": "18px",
    },
    permission: {
      "--operation-scene-enter-x": "0",
      "--operation-scene-enter-y": "0",
    },
    summary: {
      "--operation-scene-enter-x": "0",
      "--operation-scene-enter-y": "12px",
    },
    task: {
      "--operation-scene-enter-x": "10px",
      "--operation-scene-enter-y": "8px",
    },
    terminal: {
      "--operation-scene-enter-x": "0",
      "--operation-scene-enter-y": "34px",
    },
    workspace: {
      "--operation-scene-enter-x": "-24px",
      "--operation-scene-enter-y": "8px",
    },
  };

  return map[intent] as CSSProperties;
}
