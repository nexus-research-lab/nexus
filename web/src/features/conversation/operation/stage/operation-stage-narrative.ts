import { useEffect, useState } from "react";

import type { StageWindowState } from "../operation-desktop-types";
import { deriveOperationStageExperiencePhase } from "../operation-stage-experience";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "../operation-types";
import type {
  StageNarrativePhase,
  StageNarrativeState,
} from "./operation-stage-model";
import {
  countDesktopRevealEvents,
  initialRevealedWindowCount,
} from "./operation-stage-window-reveal";

const MAX_NARRATIVE_EVENTS = 24;

export function orderWindowsForReveal(
  windows: StageWindowState[],
  active_window_id: string | null,
): StageWindowState[] {
  return [...windows].sort((left, right) => {
    const left_rank = window_reveal_rank(left, active_window_id);
    const right_rank = window_reveal_rank(right, active_window_id);
    if (left_rank !== right_rank) {
      return left_rank - right_rank;
    }
    return right.z - left.z;
  });
}

function window_reveal_rank(window: StageWindowState, active_window_id: string | null): number {
  if (window.id === active_window_id || window.phase === "focused") {
    return 0;
  }
  if (window.kind === "terminal" || window.kind === "browser" || window.kind === "code_editor") {
    return 1;
  }
  if (window.kind === "run_manifest") {
    return 1;
  }
  if (window.kind === "finder" || window.layout === "artifact") {
    return 2;
  }
  if (window.kind === "evidence" || window.kind === "permission_wait") {
    return 3;
  }
  return 2;
}

export function isLowSignalDirectorValue(value: string | null | undefined): value is string {
  if (!value) {
    return true;
  }

  const normalized = value.trim().toLowerCase();
  return (
    !normalized ||
    normalized === "0 turns" ||
    normalized === "1 turns" ||
    normalized === "2 turns" ||
    normalized === "3 turns" ||
    normalized.endsWith(" turns") ||
    /^\d+\s+turns?$/.test(normalized) ||
    /^\d+\s+actions?$/.test(normalized) ||
    /^\d+\s+turns?$/.test(normalized.replace("回合", "turns")) ||
    /^\d+\s+步$/.test(normalized) ||
    normalized === "0s" ||
    normalized === "1s" ||
    normalized === "当前目标" ||
    normalized === "本轮执行收口"
  );
}

export function useRevealedWindowCount({
  event_key,
  minimum_count,
  phase,
  window_count,
}: {
  event_key: string;
  minimum_count: number;
  phase: StageNarrativePhase;
  window_count: number;
}): number {
  const [revealed_count, set_revealed_count] = useState(() => initialRevealedWindowCount({
    minimum_count,
    phase,
    window_count,
  }));

  useEffect(() => {
    if (window_count <= 0) {
      set_revealed_count(0);
      return;
    }
    if (phase === "completed" || phase === "settling") {
      set_revealed_count(window_count);
      return;
    }

    set_revealed_count(minimum_count);
    const hidden_count = Math.max(0, window_count - minimum_count);
    const timers = Array.from({ length: hidden_count }).map((_, index) => (
      window.setTimeout(() => {
        set_revealed_count((current) => Math.max(current, minimum_count + index + 1));
      }, 620 + index * 320)
    ));

    return () => {
      timers.forEach((timer) => window.clearTimeout(timer));
    };
  }, [event_key, minimum_count, phase, window_count]);

  return Math.min(revealed_count, window_count);
}

export function minimumRevealedWindowCount({
  reveal_event_count,
  phase,
  window_count,
}: {
  reveal_event_count: number;
  phase: StageNarrativePhase;
  window_count: number;
}): number {
  if (window_count <= 0) {
    return 0;
  }
  if (phase === "completed" || phase === "settling") {
    return window_count;
  }
  return Math.min(window_count, Math.max(1, reveal_event_count));
}

export { countDesktopRevealEvents };

export function buildStageNarrative(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
): StageNarrativeState {
  const events = collectNarrativeEvents(event, snapshot);
  const phase = deriveOperationStageExperiencePhase(event, snapshot);
  if (phase === "awakening") {
    return {
      phase: "awakening",
      label: event.surface === "conversation" ? "Nexus 桌面" : "唤醒桌面",
      detail: event.surface === "conversation"
        ? "桌面已唤醒，等待第一个应用窗口打开"
        : "Nexus 桌面正在展开为工作现场",
    };
  }
  if (event.phase === "waiting") {
    return {
      phase: "running",
      label: "等待确认",
      detail: "工具已暂停，等待用户确认后继续",
    };
  }
  if (phase === "running") {
    if (event.surface === "conversation") {
      return {
        phase: "running",
        label: "桌面待命",
        detail: "Nexus 桌面保持空场，等待第一个工具打开应用窗口",
      };
    }
    return {
      phase: "running",
      label: "现场执行",
      detail: `${events.length} 个工具动作正在形成桌面轨迹`,
    };
  }
  if (
    phase === "completed" ||
    (phase === "settling" && (event.phase === "done" || event.phase === "cancelled"))
  ) {
    return {
      phase,
      label: phase === "completed" ? "完成沉淀" : "结果落盘",
      detail: "应用窗口已收束为可回看的桌面现场",
    };
  }
  return {
    phase: "settling",
    label: "异常回看",
    detail: "执行现场保留错误证据与上下文",
  };
}

export function collectNarrativeEvents(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
): NexusOperationEvent[] {
  const events = snapshot?.events.filter((item) => item.round_id === event.round_id) ?? [];
  const merged = events.some((item) => item.id === event.id) ? events : [...events, event];
  const sorted = [...merged].sort((left, right) => left.updated_at - right.updated_at);
  const active_index = sorted.findIndex((item) => item.id === event.id);
  if (active_index < 0) {
    return sorted.slice(-MAX_NARRATIVE_EVENTS);
  }
  return sorted.slice(0, active_index + 1).slice(-MAX_NARRATIVE_EVENTS);
}
