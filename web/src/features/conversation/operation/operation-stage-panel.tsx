"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { Bug } from "lucide-react";

import { cn } from "@/lib/utils";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { PermissionDecisionPayload } from "@/types/conversation/permission";

import {
  build_operation_stage_key,
  useOperationStageStore,
} from "./operation-store";
import {
  operation_event_from_runtime_event,
} from "./operation-desktop-intents";
import {
  derive_operation_stage_experience_phase,
} from "./operation-stage-experience";
import {
  EmptyStage,
} from "./operation-stage-idle";
import {
  PHASE_META,
  SURFACE_META,
} from "./operation-stage-panel-style";
import {
  build_stage_transition_style,
} from "./operation-stage-transition";
import type { StageTransitionIntent } from "./operation-stage-transition";
import { OperationStageMotionStyles } from "./operation-stage-motion-styles";
import { OperationStageDesktop } from "./stage/operation-stage-desktop";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";

interface OperationStagePanelProps {
  identity: AgentConversationIdentity | null;
  agent_name?: string | null;
  header_action?: ReactNode;
  presentation?: "panel" | "stage";
}

type StageTransitionPhase = "idle" | "priming" | "materializing" | "handoff" | "live";

interface StageTransitionState {
  intent: StageTransitionIntent;
  phase: StageTransitionPhase;
  sequence: number;
}

export function OperationStagePanel({
  identity,
  agent_name,
  header_action,
  presentation = "panel",
}: OperationStagePanelProps) {
  const [is_debug_open, set_is_debug_open] = useState(false);
  const stage_key = build_operation_stage_key(identity);
  const snapshot = useOperationStageStore((state) => (
    stage_key ? state.snapshots[stage_key] : null
  ));
  const permission_response_handler = useOperationStageStore((state) => (
    stage_key ? state.permission_response_handlers[stage_key] : undefined
  ));
  const runtime_display_event = useMemo(() => {
    const runtime_event = (snapshot?.runtime_events ?? []).at(-1) ?? null;
    return runtime_event ? operation_event_from_runtime_event(runtime_event) : null;
  }, [snapshot?.runtime_events]);
  const display_event = runtime_display_event ?? snapshot?.active_event ?? snapshot?.events.at(-1) ?? null;
  const phase_meta = display_event ? PHASE_META[display_event.phase] : null;
  const PhaseIcon = phase_meta?.Icon;
  const subtitle = display_event
    ? `${agent_name || display_event.agent_id || "Agent"} / ${SURFACE_META[display_event.surface].label}`
    : agent_name || "Agent";
  const stage_surface = (
    <>
      <OperationStageMotionStyles />
      <StageSurface
        active_event={display_event}
        header_action={header_action}
        is_debug_open={is_debug_open}
        on_permission_response={permission_response_handler}
        presentation={presentation}
        snapshot={snapshot ?? null}
        subtitle={subtitle}
        on_toggle_debug={() => set_is_debug_open((value) => !value)}
      />
    </>
  );

  if (presentation === "stage") {
    return stage_surface;
  }

  return (
    <WorkspaceSurfaceView
      action={(
        <div className="flex items-center gap-2">
          <WorkspaceSurfaceToolbarAction onClick={() => set_is_debug_open((value) => !value)}>
            <Bug className="h-3.5 w-3.5" />
            证据
          </WorkspaceSurfaceToolbarAction>
          {header_action}
        </div>
      )}
      body_class_name="px-2 py-2 sm:px-3 xl:px-4"
      body_scrollable={false}
      content_class_name="flex h-full min-h-0 max-w-none"
      eyebrow="操作"
      max_width_class_name="max-w-none"
      show_eyebrow={false}
      title="操作舞台"
      title_trailing={phase_meta && PhaseIcon ? (
        <span className={cn(
          "inline-flex h-6 items-center gap-1.5 rounded-full border px-2 text-[10px] font-semibold",
          phase_meta.class_name,
        )}>
          <PhaseIcon className={cn("h-3.5 w-3.5", display_event?.phase === "running" && "animate-spin")} />
          {phase_meta.label}
        </span>
      ) : null}
    >
      {stage_surface}
    </WorkspaceSurfaceView>
  );
}

function StageSurface({
  active_event,
  snapshot,
  subtitle,
  presentation,
  header_action,
  is_debug_open,
  on_permission_response,
  on_toggle_debug,
}: {
  active_event: NexusOperationEvent | null;
  snapshot: NexusOperationSnapshot | null;
  subtitle: string;
  presentation: "panel" | "stage";
  header_action?: ReactNode;
  is_debug_open: boolean;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  on_toggle_debug: () => void;
}) {
  const is_stage = presentation === "stage";
  const stage_transition = useStageTransition(active_event);
  const is_scene_entering = stage_transition.phase === "priming" || stage_transition.phase === "materializing";
  const experience_phase = derive_operation_stage_experience_phase(active_event, snapshot);
  const transition_style = build_stage_transition_style(stage_transition.intent);
  const round_event_count = active_event && snapshot
    ? snapshot.events.filter((item) => item.round_id === active_event.round_id).length
    : active_event ? 1 : 0;
  const previous_round_event = active_event && snapshot
    ? find_previous_round_event(active_event, snapshot)
    : null;

  return (
    <section className={cn(
      "relative flex h-full min-h-[420px] w-full max-w-full min-w-0 flex-1 overflow-hidden text-(--text-strong)",
      is_stage
        ? "rounded-[24px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_78%,transparent)] p-2 shadow-[0_24px_80px_rgba(18,28,42,0.12)]"
        : "surface-panel rounded-[22px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-(--surface-panel-shadow)",
    )}
    data-stage-experience-phase={experience_phase}
    >
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(42%_30%_at_10%_8%,rgba(91,114,255,0.065),transparent_70%),radial-gradient(36%_34%_at_90%_92%,rgba(79,162,159,0.075),transparent_72%)]" />
      <div className="pointer-events-none absolute inset-x-10 top-0 h-px bg-gradient-to-r from-transparent via-white/65 to-transparent" />

      <div className="relative z-10 flex min-h-0 min-w-0 max-w-full flex-1 flex-col">
        <div className={cn("min-h-0 min-w-0 max-w-full flex-1", is_stage ? "p-0" : "px-4 pb-4 pt-4")}>
          <div className={cn(
            "relative h-full min-h-[300px] min-w-0 max-w-full overflow-hidden border border-white/60 bg-[rgba(245,248,252,0.86)] shadow-[inset_0_1px_0_rgba(255,255,255,0.84),0_30px_76px_rgba(55,70,90,0.14)]",
            is_stage ? "rounded-[20px]" : "rounded-[22px]",
          )}>
            {active_event ? (
              <>
                {is_scene_entering ? (
                  <EmptyStage
                    active_event={active_event}
                    exiting
                    key={`idle-exit-${stage_transition.sequence}`}
                    previous_event={previous_round_event}
                    round_event_count={round_event_count}
                    snapshot={snapshot}
                    subtitle={subtitle}
                    transition_intent={stage_transition.intent}
                  />
                ) : null}
                <div
                  className={cn("h-full min-h-0", is_scene_entering && "operation-stage-scene-enter")}
                  key={is_scene_entering ? `scene-enter-${stage_transition.sequence}` : "scene-live"}
                  style={is_scene_entering ? transition_style : undefined}
                >
                  <StageScene
                    event={active_event}
                    on_permission_response={on_permission_response}
                    snapshot={snapshot}
                  />
                </div>
              </>
            ) : (
              <EmptyStage snapshot={snapshot} subtitle={subtitle} />
            )}
          </div>
        </div>
      </div>

      {is_stage ? (
        <StageOverlayControls
          header_action={header_action}
          is_debug_open={is_debug_open}
          on_toggle_debug={on_toggle_debug}
        />
      ) : null}

      {is_debug_open ? (
        <DebugOverlay
          event={active_event}
          presentation={presentation}
          snapshot={snapshot}
        />
      ) : null}
    </section>
  );
}

function StageOverlayControls({
  header_action,
  is_debug_open,
  on_toggle_debug,
}: {
  header_action?: ReactNode;
  is_debug_open: boolean;
  on_toggle_debug: () => void;
}) {
  return (
    <div className="absolute right-3 top-3 z-30 flex items-center gap-1 rounded-full border border-white/70 bg-white/72 p-1 text-(--icon-default) opacity-75 shadow-[0_14px_34px_rgba(18,28,42,0.12)] backdrop-blur-xl transition-opacity hover:opacity-100 focus-within:opacity-100">
      <button
        aria-label="切换证据摘要"
        className={cn(
          "inline-flex h-7 w-7 items-center justify-center rounded-full text-(--icon-default) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
          is_debug_open && "bg-(--surface-interactive-active-background) text-(--text-strong)",
        )}
        onClick={on_toggle_debug}
        type="button"
      >
        <Bug className="h-3.5 w-3.5" />
      </button>
      {header_action ? (
        <div className="[&_button]:h-7 [&_button]:w-7 [&_button]:gap-0 [&_button]:rounded-full [&_button]:border-transparent [&_button]:bg-transparent [&_button]:p-0 [&_button]:text-[0px] [&_button]:shadow-none [&_svg]:h-3.5 [&_svg]:w-3.5">
          {header_action}
        </div>
      ) : null}
    </div>
  );
}

function StageScene({
  event,
  on_permission_response,
  snapshot,
}: {
  event: NexusOperationEvent;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  snapshot: NexusOperationSnapshot | null;
}) {
  return (
    <OperationStageDesktop
      event={event}
      on_permission_response={on_permission_response}
      snapshot={snapshot}
    />
  );
}

function find_previous_round_event(
  active_event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot,
): NexusOperationEvent | null {
  const round_events = snapshot.events
    .filter((item) => item.round_id === active_event.round_id)
    .sort((left, right) => left.updated_at - right.updated_at);
  const active_index = round_events.findIndex((item) => item.id === active_event.id);
  if (active_index <= 0) {
    return null;
  }
  return round_events[active_index - 1] ?? null;
}

function useStageTransition(active_event: NexusOperationEvent | null): StageTransitionState {
  const [transition, set_transition] = useState<StageTransitionState>(() => ({
    intent: active_event ? resolve_stage_transition_intent(active_event) : "summary",
    phase: active_event ? "priming" : "idle",
    sequence: 0,
  }));
  const previous_event_key_ref = useRef<string | null>(null);

  useEffect(() => {
    if (!active_event) {
      previous_event_key_ref.current = null;
      set_transition((current) => ({
        intent: current.intent,
        phase: "idle",
        sequence: current.sequence,
      }));
      return;
    }

    const next_event_key = build_stage_event_key(active_event);
    const next_intent = resolve_stage_transition_intent(active_event);
    const is_idle_entry = previous_event_key_ref.current === null;
    const is_same_event_state = previous_event_key_ref.current === next_event_key;
    previous_event_key_ref.current = next_event_key;

    if (is_same_event_state) {
      return;
    }

    if (!is_idle_entry) {
      let cancelled = false;
      set_transition((current) => ({
        intent: next_intent,
        phase: "handoff",
        sequence: current.sequence + 1,
      }));

      const live_timer = window.setTimeout(() => {
        if (!cancelled) {
          set_transition((current) => ({
            ...current,
            phase: "live",
          }));
        }
      }, 1400);

      return () => {
        cancelled = true;
        window.clearTimeout(live_timer);
      };
    }

    let cancelled = false;
    set_transition((current) => ({
      intent: next_intent,
      phase: "priming",
      sequence: current.sequence + 1,
    }));

    const materialize_timer = window.setTimeout(() => {
      if (!cancelled) {
        set_transition((current) => ({
          ...current,
          phase: "materializing",
        }));
      }
    }, 120);
    const live_timer = window.setTimeout(() => {
      if (!cancelled) {
        set_transition((current) => ({
          ...current,
          phase: "live",
        }));
      }
    }, 1120);

    return () => {
      cancelled = true;
      window.clearTimeout(materialize_timer);
      window.clearTimeout(live_timer);
    };
  }, [active_event]);

  return transition;
}

function build_stage_event_key(event: NexusOperationEvent): string {
  return `${event.id}:${event.phase}`;
}

function resolve_stage_transition_intent(event: NexusOperationEvent): StageTransitionIntent {
  if (event.phase === "waiting" || event.surface === "conversation" || event.kind === "human_gate") {
    return "permission";
  }
  if (event.surface === "terminal") {
    return "terminal";
  }
  if (event.surface === "web") {
    return "browser";
  }
  if (event.surface === "task") {
    return "task";
  }
  if (event.surface === "workspace") {
    return "workspace";
  }
  if (event.surface === "editor" || event.surface === "knowledge") {
    return "editor";
  }
  return "summary";
}

function DebugOverlay({
  event,
  presentation = "panel",
  snapshot,
}: {
  event: NexusOperationEvent | null;
  presentation?: "panel" | "stage";
  snapshot: NexusOperationSnapshot | null;
}) {
  return (
    <div className={cn(
      "surface-popover absolute right-4 z-20 w-[min(460px,calc(100%-2rem))] rounded-[16px] p-3",
      presentation === "stage" ? "top-14" : "top-4",
    )}>
      <div className="mb-2 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-[11px] font-semibold text-(--text-strong)">
          <Bug className="h-3.5 w-3.5" />
          证据摘要
        </div>
        <span className="text-[10px] text-(--text-soft)">{snapshot?.events.length ?? 0} events</span>
      </div>
      <pre className="soft-scrollbar max-h-[360px] overflow-auto whitespace-pre-wrap break-words rounded-[12px] border border-(--divider-subtle-color) bg-white/70 p-3 text-[10.5px] leading-5 text-(--text-default)">
        {JSON.stringify({
          active: event,
          recent_evidence: snapshot?.recent_evidence ?? [],
        }, null, 2)}
      </pre>
    </div>
  );
}
