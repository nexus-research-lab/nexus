import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";
import type { PermissionDecisionPayload } from "@/types/conversation/interaction/permission";

export { buildOperationStageKey } from "./operation-stage-key";
import type { NexusOperationSnapshot } from "./operation-types";

const MAX_PERSISTED_STAGE_SNAPSHOTS = 12;
const MAX_PERSISTED_EVENTS = 24;
const MAX_PERSISTED_WORKSPACE_EVENTS = 8;
const MAX_PERSISTED_PREVIEW_CHARS = 32000;

interface OperationStageStoreState {
  permission_response_handlers: Record<string, (payload: PermissionDecisionPayload) => boolean>;
  snapshots: Record<string, NexusOperationSnapshot>;
  set_permission_response_handler: (
    key: string,
    handler: ((payload: PermissionDecisionPayload) => boolean) | null,
  ) => void;
  set_snapshot: (key: string, snapshot: NexusOperationSnapshot) => void;
  clear_snapshot: (key: string) => void;
}

export const useOperationStageStore = create<OperationStageStoreState>()(
  persist(
    (set) => ({
      permission_response_handlers: {},
      snapshots: {},

      set_permission_response_handler: (key, handler) => {
        set((state) => {
          const { [key]: existing, ...rest } = state.permission_response_handlers;
          if (!handler) {
            if (!existing) {
              return state;
            }
            return { permission_response_handlers: rest };
          }
          if (existing === handler) {
            return state;
          }
          return {
            permission_response_handlers: {
              ...rest,
              [key]: handler,
            },
          };
        });
      },

      set_snapshot: (key, snapshot) => {
        set((state) => {
          const next_snapshot = compact_snapshot(snapshot);
          const current_snapshot = state.snapshots[key];
          if (
            current_snapshot &&
            current_snapshot.updated_at > next_snapshot.updated_at &&
            has_snapshot_content(current_snapshot)
          ) {
            return state;
          }
          if (is_snapshot_equivalent(state.snapshots[key], next_snapshot)) {
            return state;
          }
          return {
            snapshots: prune_snapshot_record({
              ...state.snapshots,
              [key]: next_snapshot,
            }),
          };
        });
      },

      clear_snapshot: (key) => {
        set((state) => {
          const { [key]: _, ...rest } = state.snapshots;
          return { snapshots: rest };
        });
      },
    }),
    {
      name: "nexus-operation-stage",
      storage: createBrowserJsonStorage(),
      version: 1,
      partialize: (state) => ({
        snapshots: prune_snapshot_record(state.snapshots),
      }),
    },
  ),
);

function prune_snapshot_record(
  snapshots: Record<string, NexusOperationSnapshot>,
): Record<string, NexusOperationSnapshot> {
  return Object.fromEntries(
    Object.entries(snapshots)
      .sort(([, left], [, right]) => right.updated_at - left.updated_at)
      .slice(0, MAX_PERSISTED_STAGE_SNAPSHOTS),
  );
}

export function compactOperationSnapshotForPersistence(snapshot: NexusOperationSnapshot): NexusOperationSnapshot {
  return compact_snapshot(snapshot);
}

function compact_snapshot(snapshot: NexusOperationSnapshot): NexusOperationSnapshot {
  return {
    ...snapshot,
    events: snapshot.events.slice(-MAX_PERSISTED_EVENTS).map((event) => ({
      ...event,
      input_preview: compact_record(event.input_preview),
      result_preview: compact_unknown(event.result_preview),
    })),
    runtime_events: (snapshot.runtime_events ?? []).slice(-MAX_PERSISTED_EVENTS).map((event) => ({
      ...event,
      artifact: event.artifact
        ? {
          ...event.artifact,
          live_content: compact_text(event.artifact.live_content, MAX_PERSISTED_PREVIEW_CHARS),
          preview: compact_unknown(event.artifact.preview),
        }
        : event.artifact,
      delta: compact_unknown(event.delta),
      input: compact_record(event.input),
      result: compact_unknown(event.result),
    })),
    recent_evidence: snapshot.recent_evidence.slice(0, 8).map((item) => ({
      ...item,
      preview: compact_unknown(item.preview),
    })),
    workspace_events: snapshot.workspace_events.slice(0, MAX_PERSISTED_WORKSPACE_EVENTS).map((item) => ({
      ...item,
      live_content: compact_text(item.live_content, MAX_PERSISTED_PREVIEW_CHARS),
    })),
  };
}

function has_snapshot_content(snapshot: NexusOperationSnapshot | undefined): boolean {
  return Boolean(
    snapshot &&
    (snapshot.events.length > 0 ||
      (snapshot.runtime_events ?? []).length > 0 ||
      snapshot.workspace_events.length > 0 ||
      snapshot.recent_evidence.length > 0 ||
      snapshot.active_event),
  );
}

function is_snapshot_equivalent(
  current: NexusOperationSnapshot | undefined,
  next: NexusOperationSnapshot,
): boolean {
  if (!current) {
    return false;
  }
  const current_last_event = current.events.at(-1);
  const next_last_event = next.events.at(-1);
  const current_runtime_events = current.runtime_events ?? [];
  const next_runtime_events = next.runtime_events ?? [];
  const current_last_runtime_event = current_runtime_events.at(-1);
  const next_last_runtime_event = next_runtime_events.at(-1);
  const current_last_workspace = current.workspace_events.at(0);
  const next_last_workspace = next.workspace_events.at(0);

  return current.active_event?.id === next.active_event?.id
    && current.active_event?.phase === next.active_event?.phase
    && current.events.length === next.events.length
    && current_last_event?.id === next_last_event?.id
    && current_last_event?.phase === next_last_event?.phase
    && current_last_event?.updated_at === next_last_event?.updated_at
    && current_runtime_events.length === next_runtime_events.length
    && current_last_runtime_event?.id === next_last_runtime_event?.id
    && current_last_runtime_event?.phase === next_last_runtime_event?.phase
    && current_last_runtime_event?.timestamp === next_last_runtime_event?.timestamp
    && current.workspace_events.length === next.workspace_events.length
    && current_last_workspace?.id === next_last_workspace?.id
    && current_last_workspace?.updated_at === next_last_workspace?.updated_at;
}

function compact_record(value: Record<string, unknown> | null | undefined): Record<string, unknown> | null | undefined {
  if (!value) {
    return value;
  }
  return Object.fromEntries(
    Object.entries(value).map(([key, item]) => [key, compact_unknown(item)]),
  );
}

function compact_unknown(value: unknown): unknown {
  if (typeof value === "string") {
    return compact_text(value, MAX_PERSISTED_PREVIEW_CHARS);
  }
  if (Array.isArray(value)) {
    return value.slice(0, 20).map((item) => compact_unknown(item));
  }
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, item]) => [key, compact_unknown(item)]),
    );
  }
  return value;
}

function compact_text(value: string | null | undefined, max_length: number): string | null | undefined {
  if (!value || value.length <= max_length) {
    return value;
  }
  return `${value.slice(0, max_length)}\n\n<!-- Nexus preview truncated for local stage persistence. -->`;
}
