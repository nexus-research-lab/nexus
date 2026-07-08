import type { KeyboardEvent, ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";

import type { PermissionDecisionPayload } from "@/types/conversation/permission";

import { StageWindowContent } from "../apps/operation-app-renderers";
import type { StageWindowState } from "../operation-desktop-types";
import type { StageWindowKind } from "../operation-desktop-types";
import {
  plan_operation_desktop,
  resolve_operation_event_window_id,
} from "../operation-scene-planner";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "../operation-types";
import {
  build_stage_narrative,
  collect_narrative_events,
  count_desktop_reveal_events,
  icon_for_window_kind,
  is_stage_desktop_window_kind,
  is_stage_manager_background_window,
  minimum_revealed_window_count,
  order_windows_for_reveal,
  position_for_window,
  stage_app_label_for_window_kind,
  useRevealedWindowCount,
  window_content_mode_for_kind,
} from "./operation-stage-helpers";
import type {
  StageWindowOverride,
} from "./operation-stage-model";
import { StageMacMenuBar, StageDesktopIcons } from "./operation-stage-mac-shell";
import { DynamicStageFrame } from "./operation-stage-frame";
import { OperationStageWindow } from "./operation-stage-window";
import {
  resolve_operation_window_keyboard_action,
  should_handle_stage_desktop_keyboard_action,
} from "./operation-stage-window-actions";
import { should_ignore_stage_desktop_keyboard_target } from "./operation-stage-keyboard-target";
import {
  StageWindowDock,
} from "./operation-stage-window-controls";
import {
  resolve_cycled_window_focus,
  resolve_next_window_focus,
} from "./operation-stage-window-focus";
import {
  is_meaningful_stage_window_drag,
  normalize_stage_window_drag_offset,
  normalize_stage_window_resize_size,
} from "./operation-stage-window-drag";
import { build_stage_window_launch_state } from "./operation-stage-window-launch";
import { OperationStageIdleDesktop } from "./operation-stage-idle-desktop";

export function OperationStageDesktop({
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
  const [focused_window_id, set_focused_window_id] = useState<string | null>(null);
  const [replay_event_id, set_replay_event_id] = useState<string | null>(null);
  const [window_overrides, set_window_overrides] = useState<Record<string, StageWindowOverride>>({});
  const narrative = useMemo(() => build_stage_narrative(event, snapshot), [event, snapshot]);
  const narrative_events = useMemo(() => collect_narrative_events(event, snapshot), [event, snapshot]);
  const active_narrative_event_id = useMemo(() => (
    replay_event_id && narrative_events.some((item) => item.id === replay_event_id)
      ? replay_event_id
      : event.id
  ), [event.id, narrative_events, replay_event_id]);
  const active_narrative_event = useMemo(() => (
    narrative_events.find((item) => item.id === active_narrative_event_id) ?? event
  ), [active_narrative_event_id, event, narrative_events]);
  const desktop = useMemo(() => (
    plan_operation_desktop({ event: active_narrative_event, snapshot })
  ), [active_narrative_event, snapshot]);
  const desktop_windows = useMemo(() => (
    desktop.windows.filter((window) => is_stage_desktop_window_kind(window.kind))
  ), [desktop.windows]);
  const stage_windows = useMemo(() => (
    desktop_windows
  ), [desktop_windows]);
  const planned_active_window_id = useMemo(() => (
    desktop_windows.some((window) => window.id === desktop.active_window_id)
      ? desktop.active_window_id
      : stage_windows[0]?.id ?? null
  ), [desktop.active_window_id, desktop_windows, stage_windows]);
  const desktop_active_window_id = useMemo(() => (
    stage_windows.some((window) => window.id === focused_window_id)
      ? focused_window_id
      : desktop_windows.some((window) => window.id === desktop.active_window_id)
      ? desktop.active_window_id
      : stage_windows[0]?.id ?? null
  ), [desktop.active_window_id, desktop_windows, focused_window_id, stage_windows]);
  const windows_for_reveal = useMemo(() => (
    order_windows_for_reveal(stage_windows, desktop_active_window_id)
  ), [desktop_active_window_id, stage_windows]);
  const reveal_event_count = useMemo(() => (
    count_desktop_reveal_events(narrative_events)
  ), [narrative_events]);
  const revealed_window_count = useRevealedWindowCount({
    event_key: `${active_narrative_event.round_id}:${active_narrative_event.id}:${active_narrative_event.phase}`,
    minimum_count: minimum_revealed_window_count({
      phase: narrative.phase,
      reveal_event_count,
      window_count: windows_for_reveal.length,
    }),
    phase: narrative.phase,
    window_count: windows_for_reveal.length,
  });

  useEffect(() => {
    set_focused_window_id(null);
    set_replay_event_id(null);
    set_window_overrides({});
  }, [event.round_id]);

  useEffect(() => {
    set_replay_event_id(null);
  }, [event.id]);

  useEffect(() => {
    const next_active_window_id = planned_active_window_id;
    if (!next_active_window_id) {
      return;
    }

    set_focused_window_id(next_active_window_id);
    set_window_overrides((current) => ({
      ...current,
      [next_active_window_id]: {
        ...current[next_active_window_id],
        closed: false,
        minimized: false,
      },
    }));
  }, [active_narrative_event.id, planned_active_window_id]);

  const window_states = useMemo(() => (
    windows_for_reveal
      .map((window): StageWindowState => {
        const override = window_overrides[window.id];
        if (override?.closed) {
          return { ...window, phase: "closed" };
        }
        if (override?.minimized) {
          return { ...window, phase: "minimized" };
        }
        if (override?.minimized === false && override.restore_token && window.phase === "minimized") {
          return { ...window, phase: "background" };
        }
        if (focused_window_id === window.id && window.phase !== "closed" && window.phase !== "minimized") {
          return { ...window, phase: "focused" };
        }
        return window;
      })
      .slice(0, revealed_window_count)
      .sort((left, right) => {
        const left_z = left.id === focused_window_id ? 100 : left.z;
        const right_z = right.id === focused_window_id ? 100 : right.z;
        return left_z - right_z;
      })
  ), [focused_window_id, revealed_window_count, window_overrides, windows_for_reveal]);

  const visible_windows = useMemo(() => (
    window_states.filter((window) => window.phase !== "closed" && window.phase !== "minimized")
  ), [window_states]);

  const active_window_id = useMemo(() => {
    if (focused_window_id && visible_windows.some((window) => (
      window.id === focused_window_id && window.phase !== "minimized"
    ))) {
      return focused_window_id;
    }
    const explicit_active = visible_windows.find((window) => (
      window.id === desktop_active_window_id && window.phase !== "minimized"
    ));
    const focused = explicit_active ?? visible_windows.find((window) => window.phase === "focused");
    return (focused ?? visible_windows[0] ?? null)?.id ?? null;
  }, [desktop_active_window_id, focused_window_id, visible_windows]);

  const active_window = useMemo(() => (
    visible_windows.find((window) => window.id === active_window_id) ?? null
  ), [active_window_id, visible_windows]);
  const has_maximized_window = visible_windows.some((window) => window_overrides[window.id]?.maximized);
  const close_window = (window_id: string) => {
    set_focused_window_id((current) => resolve_next_window_focus({
      current_focus_id: current,
      hidden_window_id: window_id,
      windows: window_states,
    }));
    set_window_overrides((current) => ({
      ...current,
      [window_id]: {
        ...current[window_id],
        closed: true,
      },
    }));
  };

  const focus_window = (window_id: string) => {
    set_focused_window_id(window_id);
    set_window_overrides((current) => ({
      ...current,
      [window_id]: {
        ...current[window_id],
        minimized: false,
      },
    }));
  };

  const minimize_window = (window_id: string) => {
    set_focused_window_id((current) => resolve_next_window_focus({
      current_focus_id: current,
      hidden_window_id: window_id,
      windows: window_states,
    }));
    set_window_overrides((current) => ({
      ...current,
      [window_id]: {
        ...current[window_id],
        minimized: true,
      },
    }));
  };

  const move_window = (window_id: string, offset: { x: number; y: number }) => {
    const normalized_offset = normalize_stage_window_drag_offset(offset);
    set_focused_window_id(window_id);
    set_window_overrides((current) => ({
      ...current,
      [window_id]: {
        ...current[window_id],
        maximized: is_meaningful_stage_window_drag(normalized_offset) ? false : current[window_id]?.maximized,
        minimized: false,
        offset_x: normalized_offset.x,
        offset_y: normalized_offset.y,
      },
    }));
  };

  const resize_window = (window_id: string, size: { height: number; width: number }) => {
    const normalized_size = normalize_stage_window_resize_size(size);
    set_focused_window_id(window_id);
    set_window_overrides((current) => ({
      ...current,
      [window_id]: {
        ...current[window_id],
        maximized: false,
        minimized: false,
        resize_height: normalized_size.height,
        resize_width: normalized_size.width,
      },
    }));
  };

  const toggle_zoom_window = (window_id: string) => {
    set_focused_window_id(window_id);
    set_window_overrides((current) => {
      const current_override = current[window_id];
      const next_maximized = !current_override?.maximized;
      return {
        ...current,
        [window_id]: {
          ...current_override,
          closed: false,
          maximized: next_maximized,
          minimized: false,
          offset_x: next_maximized ? 0 : current_override?.offset_x,
          offset_y: next_maximized ? 0 : current_override?.offset_y,
        },
      };
    });
  };

  const cycle_window_focus = (direction: "next" | "previous") => {
    set_focused_window_id((current) => resolve_cycled_window_focus({
      current_focus_id: current ?? active_window_id,
      direction,
      windows: window_states,
    }));
  };

  const handle_desktop_key_down = (keyboard_event: KeyboardEvent<HTMLDivElement>) => {
    if (is_text_entry_keyboard_target(keyboard_event.target)) {
      return;
    }
    const action = resolve_operation_window_keyboard_action(keyboard_event);
    if (!action || !should_handle_stage_desktop_keyboard_action(action)) {
      return;
    }
    keyboard_event.preventDefault();
    keyboard_event.stopPropagation();
    if (action === "cycle_next") {
      cycle_window_focus("next");
    } else if (action === "cycle_previous") {
      cycle_window_focus("previous");
    } else if (active_window_id && action === "close") {
      close_window(active_window_id);
    } else if (active_window_id && action === "minimize") {
      minimize_window(active_window_id);
    } else if (active_window_id && action === "zoom") {
      toggle_zoom_window(active_window_id);
    }
  };

  const restore_window = (window_id: string) => {
    set_focused_window_id(window_id);
    const restore_token = Date.now();
    set_window_overrides((current) => ({
      ...current,
      [window_id]: {
        ...current[window_id],
        closed: false,
        minimized: false,
        restore_token,
      },
    }));
  };

  const restore_all_windows = () => {
    set_focused_window_id(desktop_active_window_id ?? stage_windows[0]?.id ?? null);
    const restore_token = Date.now();
    set_window_overrides(Object.fromEntries(
      stage_windows.map((window, index) => [window.id, {
        closed: false,
        minimized: false,
        restore_token: restore_token + index,
      }]),
    ));
  };

  const focus_event_window = (target_event: NexusOperationEvent) => {
    const target_window_id = resolve_operation_event_window_id(target_event, desktop_windows)
      ?? desktop_active_window_id
      ?? desktop_windows[0]?.id
      ?? null;
    if (!target_window_id) {
      return;
    }
    set_replay_event_id(target_event.id);
    restore_window(target_window_id);
  };

  if (!stage_windows.length) {
    return (
      <OperationStageIdleDesktop
        header_action={header_action}
        presentation="stage"
      />
    );
  }

  return (
    <DynamicStageFrame
      event={event}
      narrative={narrative}
      on_key_down_capture={handle_desktop_key_down}
    >
      <StageMacMenuBar
        active_event={active_narrative_event}
        active_window={active_window}
        events={narrative_events}
        header_action={header_action}
        on_focus_event={focus_event_window}
        windows={window_states}
      />
      <StageDesktopIcons windows={window_states} on_restore={restore_window} />
      {visible_windows.length ? visible_windows.map((window, index) => {
        const window_override = window_overrides[window.id];
        const is_active = active_window_id === window.id && window.phase !== "minimized";
        const is_maximized = Boolean(window_override?.maximized);
        const background_window_index = visible_windows
          .filter((item) => is_stage_manager_background_window(item, narrative.phase))
          .findIndex((item) => item.id === window.id);
        const is_stage_manager_preview = is_stage_manager_background_window(window, narrative.phase);
        const launch = build_stage_window_launch_state({ index, is_active, window });
        return (
          <OperationStageWindow
            app_label={stage_app_label_for_window_kind(window.kind)}
            delay_ms={launch.delay_ms}
            dimmed={!is_active && window.phase !== "minimized"}
            drag_offset={is_maximized ? { x: 0, y: 0 } : {
              x: window_override?.offset_x ?? 0,
              y: window_override?.offset_y ?? 0,
            }}
            focus={is_active}
            icon={icon_for_window_kind(window.kind)}
            key={window.id}
            content_mode={window_content_mode_for_kind(window.kind)}
            launch_origin={launch.origin}
            maximized={is_maximized}
            mobile_hidden={!is_active}
            minimized={window.phase === "minimized"}
            on_close={() => close_window(window.id)}
            on_drag={(offset) => move_window(window.id, offset)}
            on_focus={() => focus_window(window.id)}
            on_minimize={() => minimize_window(window.id)}
            on_resize={(size) => resize_window(window.id, size)}
            on_zoom={() => toggle_zoom_window(window.id)}
            on_cycle_focus={cycle_window_focus}
            position_class_name={is_maximized
              ? "inset-x-4 top-14 bottom-0 h-auto w-auto"
              : position_for_window(window, narrative.phase, background_window_index)}
            preview_mode={is_stage_manager_preview ? "stage-manager" : undefined}
            resize_size={!is_maximized && window_override?.resize_width && window_override.resize_height ? {
              height: window_override.resize_height,
              width: window_override.resize_width,
            } : undefined}
            restore_token={window_override?.restore_token}
            title={window.title}
            tone={window.kind === "terminal" ? "terminal" : "default"}
            z_index={is_active ? 44 : 8 + index}
          >
            <StageWindowContent
              window={window}
              on_focus_event={is_active ? focus_event_window : undefined}
              on_permission_response={on_permission_response}
            />
          </OperationStageWindow>
        );
      }) : null}
      {has_maximized_window ? null : (
        <StageWindowDock
          active_window_id={active_window_id}
          on_restore_all={restore_all_windows}
          windows={window_states}
          on_restore={restore_window}
        />
      )}
    </DynamicStageFrame>
  );
}

function is_text_entry_keyboard_target(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  if (should_ignore_stage_desktop_keyboard_target({
    content_editable: target.getAttribute("contenteditable"),
    is_content_editable: target.isContentEditable,
    tag_name: target.tagName,
  })) {
    return true;
  }
  return Boolean(target.closest("input, textarea, select, [contenteditable='true']"));
}
