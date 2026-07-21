import { useEffect, useLayoutEffect, useMemo, useRef } from "react";

import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { Message } from "@/types/conversation/message/entity";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import {
  getOperationStageSnapshotApi,
  saveOperationStageSnapshotApi,
} from "./operation-stage-api";
import { projectOperationSnapshot } from "./operation-projector";
import {
  mergeOperationStageSnapshotsForRestore,
  sanitizeOperationStageSnapshotForRestore,
} from "./operation-stage-snapshot-merge";
import {
  buildOperationStageKey,
  compactOperationSnapshotForPersistence,
  useOperationStageStore,
} from "./operation-store";

interface UseOperationProjectionSyncParams {
  identity: AgentConversationIdentity | null;
  messages: Message[];
  pending_permissions: PendingPermission[];
  live_round_ids: string[];
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
}

export function useOperationProjectionSync({
  identity,
  messages,
  pending_permissions,
  live_round_ids,
  on_permission_response,
}: UseOperationProjectionSyncParams): void {
  const key = buildOperationStageKey(identity);
  const recent_workspace_events = useWorkspaceLiveStore((state) => state.recent_events);
  const set_snapshot = useOperationStageStore((state) => state.set_snapshot);
  const set_permission_response_handler = useOperationStageStore((state) => state.set_permission_response_handler);
  const last_saved_signature_ref = useRef<string | null>(null);
  const active_stage_key_ref = useRef<string | null>(key);
  const workspace_event_floor_ref = useRef(Date.now());

  if (active_stage_key_ref.current !== key) {
    active_stage_key_ref.current = key;
    workspace_event_floor_ref.current = Date.now();
  }
  const workspace_event_floor = workspace_event_floor_ref.current;

  useEffect(() => {
    last_saved_signature_ref.current = null;
  }, [key]);

  useEffect(() => {
    if (!key) {
      return;
    }
    set_permission_response_handler(key, on_permission_response ?? null);
    return () => set_permission_response_handler(key, null);
  }, [key, on_permission_response, set_permission_response_handler]);

  const scoped_workspace_events = useMemo(() => {
    const session_key = identity?.session_key ?? null;

    return recent_workspace_events.filter((event) => {
      if (event.session_key) {
        return areEquivalentSessionKeys(event.session_key, session_key);
      }
      return event.updated_at >= workspace_event_floor;
    });
  }, [
    identity?.session_key,
    recent_workspace_events,
    workspace_event_floor,
  ]);

  const snapshot = useMemo(() => {
    if (!key) {
      return null;
    }

    return projectOperationSnapshot({
      key,
      session_key: identity?.session_key ?? null,
      agent_id: identity?.agent_id ?? null,
      messages,
      pending_permissions,
      live_round_ids,
      workspace_events: scoped_workspace_events,
    });
  }, [
    identity?.agent_id,
    identity?.session_key,
    key,
    live_round_ids,
    messages,
    pending_permissions,
    scoped_workspace_events,
  ]);

  useEffect(() => {
    if (!key) {
      return;
    }

    let cancelled = false;
    void getOperationStageSnapshotApi(key).then((remote_snapshot) => {
      if (!cancelled && remote_snapshot) {
        const current_snapshot = useOperationStageStore.getState().snapshots[key];
        const restored_snapshot = sanitizeOperationStageSnapshotForRestore(remote_snapshot);
        set_snapshot(
          key,
          current_snapshot
            ? mergeOperationStageSnapshotsForRestore(restored_snapshot, current_snapshot)
            : restored_snapshot,
        );
      }
    });
    return () => {
      cancelled = true;
    };
  }, [key, set_snapshot]);

  useLayoutEffect(() => {
    if (!key || !snapshot) {
      return;
    }

    const current_snapshot = useOperationStageStore.getState().snapshots[key];
    const merged_snapshot = mergeOperationStageSnapshotsForRestore(current_snapshot, snapshot);

    set_snapshot(key, merged_snapshot);
  }, [key, set_snapshot, snapshot]);

  useEffect(() => {
    if (!key || !snapshot) {
      return;
    }

    const current_snapshot = useOperationStageStore.getState().snapshots[key];
    if (!current_snapshot) {
      return;
    }
    const compact_snapshot = compactOperationSnapshotForPersistence(current_snapshot);
    const signature = build_snapshot_signature(compact_snapshot);
    if (!signature || last_saved_signature_ref.current === signature) {
      return;
    }

    last_saved_signature_ref.current = signature;
    const timer = window.setTimeout(() => {
      void saveOperationStageSnapshotApi(key, compact_snapshot);
    }, 650);
    return () => window.clearTimeout(timer);
  }, [key, snapshot]);
}

type CompactOperationSnapshot = ReturnType<typeof compactOperationSnapshotForPersistence>;

function build_snapshot_signature(snapshot: CompactOperationSnapshot): string | null {
  const runtime_events = snapshot.runtime_events ?? [];
  if (
    snapshot.events.length === 0 &&
    runtime_events.length === 0 &&
    snapshot.workspace_events.length === 0 &&
    snapshot.recent_evidence.length === 0 &&
    !snapshot.active_event
  ) {
    return null;
  }
  const active = snapshot.active_event;
  const last_event = snapshot.events.at(-1);
  const last_runtime_event = runtime_events.at(-1);
  const last_workspace_event = snapshot.workspace_events.at(0);
  return [
    snapshot.updated_at,
    active?.id ?? "",
    active?.phase ?? "",
    last_event?.id ?? "",
    last_event?.phase ?? "",
    last_runtime_event?.id ?? "",
    last_runtime_event?.phase ?? "",
    last_runtime_event?.timestamp ?? "",
    last_workspace_event?.id ?? "",
    last_workspace_event?.updated_at ?? "",
  ].join(":");
}
