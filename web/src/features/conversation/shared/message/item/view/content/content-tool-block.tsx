import type { ReactNode } from "react";

import type { ToolUseContent } from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
  PermissionUpdate,
} from "@/types/conversation/permission";

import { AskUserQuestionBlock } from "../../../blocks/question/ask-user-question-block";
import { ToolBlock } from "../../../blocks/tool/tool-block";
import type { ToolPermissionRequest } from "../../../blocks/tool/tool-block-types";
import {
  resolveToolBlockStatus,
  type StructuredContentProjection,
} from "./content-renderer-model";

interface ContentToolBlockOptions {
  block: ToolUseContent;
  canRespondToPermissions: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingPermission?: PendingPermission;
  permissionReadOnlyReason?: string;
  projection: StructuredContentProjection;
  workspaceAgentId?: string | null;
}

export function renderContentToolBlock({
  block,
  canRespondToPermissions,
  onOpenWorkspaceFile,
  onPermissionResponse,
  pendingPermission,
  permissionReadOnlyReason,
  projection,
  workspaceAgentId,
}: ContentToolBlockOptions): ReactNode {
  const toolUse = projection.toolUseById.get(block.id);
  const waitingForPermission = Boolean(pendingPermission && !toolUse?.result);

  if (block.name === "AskUserQuestion") {
    return (
      <AskUserQuestionBlock
        initialSubmitted={Boolean(toolUse?.result && !toolUse.result.is_error)}
        interactionDisabled={!canRespondToPermissions}
        isReady={waitingForPermission}
        onSubmit={(_, answers) => {
          if (!pendingPermission) {
            return false;
          }
          return onPermissionResponse?.({
            decision: "allow",
            request_id: pendingPermission.request_id,
            user_answers: answers,
          }) ?? false;
        }}
        toolResult={toolUse?.result}
        toolUse={block}
      />
    );
  }

  return (
    <div className="min-w-0">
      <ToolBlock
        interactionDisabled={!canRespondToPermissions}
        interactionDisabledReason={permissionReadOnlyReason}
        liveProgress={projection.taskProgressByToolUseId.get(block.id) ?? null}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        permissionRequest={pendingPermission && waitingForPermission
          ? createPermissionRequest(pendingPermission, onPermissionResponse)
          : undefined}
        status={resolveToolBlockStatus(toolUse, waitingForPermission)}
        toolResult={toolUse?.result}
        toolUse={block}
        workspaceAgentId={workspaceAgentId}
      />
    </div>
  );
}

function createPermissionRequest(
  permission: PendingPermission,
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean,
): ToolPermissionRequest {
  const respond = (
    decision: PermissionDecisionPayload["decision"],
    updatedPermissions?: PermissionUpdate[],
  ) => onPermissionResponse?.({
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
