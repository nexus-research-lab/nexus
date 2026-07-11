import { cn } from "@/lib/utils";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

import { ToolBlock } from "../../../blocks/tool/tool-block";

interface PendingPermissionListProps {
  canRespond: boolean;
  isRoomThreadMode: boolean;
  onResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissions: PendingPermission[];
  readOnlyReason?: string;
  workspaceAgentId?: string | null;
}

export function PendingPermissionList({
  canRespond,
  isRoomThreadMode,
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
        isRoomThreadMode
          ? "border-t border-(--divider-subtle-color) pt-3"
          : "rounded-2xl bg-transparent p-3",
      )}
    >
      {permissions.map((permission) => (
        <ToolBlock
          interactionDisabled={!canRespond}
          interactionDisabledReason={readOnlyReason}
          key={permission.request_id}
          permissionRequest={{
            expires_at: permission.expires_at,
            on_allow: (updatedPermissions) => onResponse?.({
              decision: "allow",
              request_id: permission.request_id,
              updated_permissions: updatedPermissions,
            }),
            on_deny: (updatedPermissions) => onResponse?.({
              decision: "deny",
              request_id: permission.request_id,
              updated_permissions: updatedPermissions,
            }),
            request_id: permission.request_id,
            risk_label: permission.risk_label,
            risk_level: permission.risk_level,
            suggestions: permission.suggestions,
            summary: permission.summary,
            tool_input: permission.tool_input,
          }}
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
