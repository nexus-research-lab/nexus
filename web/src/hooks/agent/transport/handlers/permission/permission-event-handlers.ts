import type { PendingPermission } from "@/types/conversation/interaction/permission";

import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../../agent-event-context";
import { withCurrentSessionEvent } from "../handler-scope";
import {
  decodePermissionRequest,
  decodeResolvedPermissionRequestId,
} from "./permission-event-data";

const handlePermissionRequest: AgentEventHandler = withCurrentSessionEvent((
  event,
  context,
) => {
  const permission = decodePermissionRequest(event);
  if (!permission) {
    return;
  }
  context.state.setPendingPermissions((current) =>
    upsertPendingPermission(current, permission));
});

const handlePermissionResolved: AgentEventHandler = withCurrentSessionEvent((
  event,
  context,
) => {
  const requestId = decodeResolvedPermissionRequestId(event);
  if (!requestId) {
    return;
  }
  context.state.setPendingPermissions((current) =>
    removePendingPermission(current, requestId));
});

function upsertPendingPermission(
  current: PendingPermission[],
  permission: PendingPermission,
): PendingPermission[] {
  return [
    ...current.filter((item) => item.request_id !== permission.request_id),
    permission,
  ];
}

function removePendingPermission(
  current: PendingPermission[],
  requestId: string,
): PendingPermission[] {
  const next = current.filter((permission) =>
    permission.request_id !== requestId);
  return next.length === current.length ? current : next;
}

export const AGENT_PERMISSION_EVENT_HANDLERS: AgentEventHandlerMap = {
  permission_request: handlePermissionRequest,
  permission_request_resolved: handlePermissionResolved,
};
