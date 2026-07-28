/**
 * INPUT: 尚未匹配到消息块、会让 runtime 等待用户响应的交互请求。
 * OUTPUT: 可直接批准、拒绝或回答的 Room 人工介入列表，并标记完整交互边界。
 * POS: Room 公区与 Thread 共用的 request-owned human-in-the-loop 投影入口；DM 由 Composer 接管。
 */
import { cn } from "@/shared/ui/class-name";
import type {
  PendingPermission,
  PermissionDecisionPayload,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";

import { ASK_USER_QUESTION_TOOL_NAME } from "../../../message-tool-names";
import { PendingHumanQuestion } from "../../../blocks/question/pending-human-question";
import { ToolBlock } from "../../../blocks/tool/tool-block";
import type { ToolPermissionRequest } from "../../../blocks/tool/tool-block-types";
import type { AssistantContentMode } from "../../message-item-projection";

interface PendingHumanInteractionListProps {
  canRespond: boolean;
  mode: AssistantContentMode;
  onResponse?: (payload: PermissionDecisionPayload) => boolean;
  permissions: PendingPermission[];
  readOnlyReason?: string;
  workspaceAgentId?: string | null;
}

export function PendingHumanInteractionList({
  canRespond,
  mode,
  onResponse,
  permissions,
  readOnlyReason,
  workspaceAgentId,
}: PendingHumanInteractionListProps) {
  if (permissions.length === 0) {
    return null;
  }

  return (
    <div
      className={cn(
        "mt-3 flex flex-col gap-3",
        HUMAN_INTERACTION_LIST_LAYOUTS[mode],
      )}
      data-human-interaction-surface
    >
      {permissions.map((permission) => (
        <div
          data-pending-interaction-request-id={permission.request_id}
          key={permission.request_id}
        >
          {isStructuredInputInteraction(permission) ? (
            <PendingHumanQuestion
              canRespond={canRespond}
              onResponse={onResponse}
              permission={permission}
              readOnlyReason={readOnlyReason}
            />
          ) : (
            <ToolBlock
              interactionDisabled={!canRespond}
              interactionDisabledReason={readOnlyReason}
              permissionRequest={createPermissionRequest(permission, onResponse)}
              status="waiting_permission"
              toolUse={{
                id: pendingToolUseId(permission),
                input: permission.tool_input,
                name: permission.tool_name,
                type: "tool_use",
              }}
              workspaceAgentId={workspaceAgentId}
            />
          )}
        </div>
      ))}
    </div>
  );
}

const HUMAN_INTERACTION_LIST_LAYOUTS: Record<AssistantContentMode, string> = {
  dm_archived: "surface-radius-md bg-transparent p-3",
  dm_live: "surface-radius-md bg-transparent p-3",
  room_result: "border-t border-(--divider-subtle-color) pt-3",
  room_thread: "border-t border-(--divider-subtle-color) pt-3",
};

function isStructuredInputInteraction(
  permission: PendingPermission,
): boolean {
  return permission.interaction_mode === "question"
    || permission.tool_name === ASK_USER_QUESTION_TOOL_NAME;
}

function pendingToolUseId(permission: PendingPermission): string {
  return `pending_${permission.request_id}`;
}

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
