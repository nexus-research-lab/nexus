import { cn } from "@/shared/ui/class-name";
import type {
  PendingPermission,
  PermissionDecisionPayload,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";

import { ToolBlock } from "../../../blocks/tool/tool-block";
import type { ToolPermissionRequest } from "../../../blocks/tool/tool-block-types";
import type { AssistantContentMode } from "../../message-item-projection";

interface PendingPermissionListProps {
  canRespond: boolean;
  mode: AssistantContentMode;
  onResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissions: PendingPermission[];
  readOnlyReason?: string;
  workspaceAgentId?: string | null;
}

export function PendingPermissionList({
  canRespond,
  mode,
  onResponse,
  permissions,
  readOnlyReason,
  workspaceAgentId,
}: PendingPermissionListProps) {
  if (permissions.length === 0) {
    return null;
  }

  return (
    <div
      className={cn(
        "mt-3 flex flex-col gap-3",
        PERMISSION_LIST_LAYOUTS[mode],
      )}
    >
      {permissions.map((permission) => (
        <ToolBlock
          interactionDisabled={!canRespond}
          interactionDisabledReason={readOnlyReason}
          key={permission.request_id}
          permissionRequest={createPermissionRequest(permission, onResponse)}
          status="waiting_permission"
          toolUse={{
            id: `pending_${permission.request_id}`,
            input: permission.tool_input,
            name: permission.tool_name,
            type: "tool_use",
          }}
          workspaceAgentId={workspaceAgentId}
        />
      ))}
    </div>
  );
}

const PERMISSION_LIST_LAYOUTS: Record<AssistantContentMode, string> = {
  dm_archived: "rounded-2xl bg-transparent p-3",
  dm_live: "rounded-2xl bg-transparent p-3",
  room_result: "rounded-2xl bg-transparent p-3",
  room_thread: "border-t border-(--divider-subtle-color) pt-3",
};

function createPermissionRequest(
  permission: PendingPermission,
  onResponse?: (payload: PermissionDecisionPayload) => boolean,
): ToolPermissionRequest {
  const respond = (
    decision: PermissionDecisionPayload["decision"],
    updatedPermissions?: PermissionUpdate[],
  ) => onResponse?.({
    decision,
    request_id: permission.request_id,
    updated_permissions: updatedPermissions,
  });
  return {
    expires_at: permission.expires_at,
    on_allow: (updatedPermissions) => respond("allow", updatedPermissions),
    on_deny: (updatedPermissions) => respond("deny", updatedPermissions),
    request_id: permission.request_id,
    risk_label: permission.risk_label,
    risk_level: permission.risk_level,
    suggestions: permission.suggestions,
    summary: permission.summary,
    tool_input: permission.tool_input,
  };
}
