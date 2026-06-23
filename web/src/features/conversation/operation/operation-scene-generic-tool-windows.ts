import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";
import type {
  StageWindowState,
} from "./operation-desktop-types";
import { generic_tool_window_config } from "./operation-scene-generic-tool-window";

interface AppendGenericToolWindowsParams {
  active_event: NexusOperationEvent;
  snapshot: NexusOperationSnapshot | null;
  tool_activity_events: NexusOperationEvent[];
  windows: StageWindowState[];
  window_state: (
    event: NexusOperationEvent,
    snapshot: NexusOperationSnapshot | null,
    config: ReturnType<typeof generic_tool_window_config>,
  ) => StageWindowState;
}

export function append_generic_tool_windows({
  active_event,
  snapshot,
  tool_activity_events,
  windows,
  window_state,
}: AppendGenericToolWindowsParams): void {
  const represented_event_ids = new Set(windows.flatMap((window) => [
    window.payload.event.id,
    ...(window.payload.related_events ?? []).map((item) => item.id),
  ]));
  const represented_tool_use_ids = new Set(windows.flatMap((window) => [
    window.payload.event.tool_use_id,
    ...(window.payload.related_events ?? []).map((item) => item.tool_use_id),
  ].filter((tool_use_id): tool_use_id is string => Boolean(tool_use_id))));
  const generic_events = tool_activity_events.filter((item) => (
    !represented_event_ids.has(item.id) &&
    !(item.tool_use_id && represented_tool_use_ids.has(item.tool_use_id))
  ));
  if (!generic_events.length) {
    return;
  }

  const recent_generic_events = generic_events.slice(-3);
  const active_generic_event = recent_generic_events.find((item) => item.id === active_event.id)
    ?? (windows.length === 0 ? recent_generic_events.at(-1) : null);

  recent_generic_events.forEach((generic_event, index) => {
    const is_active = active_generic_event?.id === generic_event.id;
    windows.push(window_state(generic_event, snapshot, generic_tool_window_config(
      generic_event,
      recent_generic_events,
      {
        phase: is_active ? "focused" : "background",
        z: is_active ? 40 : 16 + index,
      },
    )));
  });
}
