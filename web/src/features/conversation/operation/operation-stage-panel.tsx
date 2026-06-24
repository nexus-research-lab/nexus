"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
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
  plan_operation_desktop,
} from "./operation-scene-planner";
import {
  derive_operation_stage_experience_phase,
} from "./operation-stage-experience";
import {
  PHASE_META,
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
  const display_event_candidate = runtime_display_event ?? snapshot?.active_event ?? snapshot?.events.at(-1) ?? null;
  const display_event = useMemo(() => (
    display_event_candidate && plan_operation_desktop({
      event: display_event_candidate,
      snapshot: snapshot ?? null,
    }).windows.length > 0
      ? display_event_candidate
      : null
  ), [display_event_candidate, snapshot]);
  const phase_meta = display_event ? PHASE_META[display_event.phase] : null;
  const PhaseIcon = phase_meta?.Icon;
  const stage_surface = (
    <>
      <OperationStageMotionStyles />
      <StageSurface
        active_event={display_event}
        header_action={presentation === "stage" ? header_action : undefined}
        on_permission_response={permission_response_handler}
        presentation={presentation}
        snapshot={snapshot ?? null}
      />
    </>
  );

  if (presentation === "stage") {
    return stage_surface;
  }

  return (
    <WorkspaceSurfaceView
      action={header_action}
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
  presentation,
  header_action,
  on_permission_response,
}: {
  active_event: NexusOperationEvent | null;
  snapshot: NexusOperationSnapshot | null;
  presentation: "panel" | "stage";
  header_action?: ReactNode;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
}) {
  const is_stage = presentation === "stage";
  const stage_transition = useStageTransition(active_event);
  const is_scene_entering = stage_transition.phase === "priming" || stage_transition.phase === "materializing";
  const experience_phase = derive_operation_stage_experience_phase(active_event, snapshot);
  const transition_style = build_stage_transition_style(stage_transition.intent);

  if (!active_event) {
    return null;
  }

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
                <div
                  className={cn("h-full min-h-0", is_scene_entering && "operation-stage-scene-enter")}
                  key={is_scene_entering ? `scene-enter-${stage_transition.sequence}` : "scene-live"}
                  style={is_scene_entering ? transition_style : undefined}
                >
                  <StageScene
                    event={active_event}
                    header_action={header_action}
                    on_permission_response={on_permission_response}
                    snapshot={snapshot}
                  />
                </div>
              </>
            ) : null}
          </div>
        </div>
      </div>
    </section>
  );
}

function StageScene({
  event,
  header_action,
  on_permission_response,
  snapshot,
}: {
  event: NexusOperationEvent;
  header_action?: ReactNode;
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  snapshot: NexusOperationSnapshot | null;
}) {
  return (
    <OperationStageDesktop
      event={event}
      header_action={header_action}
      on_permission_response={on_permission_response}
      snapshot={snapshot}
    />
  );
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
