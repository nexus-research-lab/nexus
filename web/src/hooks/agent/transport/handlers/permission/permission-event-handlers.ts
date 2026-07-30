import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../../agent-event-context";
import { withCurrentSessionEvent } from "../handler-scope";
import {
  decodePermissionRequest,
  decodeResolvedPermissionRequestId,
} from "./permission-event-data";
import {
  removePendingPermission,
  upsertPendingPermission,
} from "./pending-permission-state";

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
  context.runtime.acknowledgePermissionRequest(requestId);
  context.state.setPendingPermissions((current) =>
    removePendingPermission(current, requestId));
});

export const AGENT_PERMISSION_EVENT_HANDLERS: AgentEventHandlerMap = {
  permission_request: handlePermissionRequest,
  permission_request_resolved: handlePermissionResolved,
};
