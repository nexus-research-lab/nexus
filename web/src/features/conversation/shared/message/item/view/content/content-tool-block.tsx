/**
 * INPUT: tool block、精确匹配请求、execution terminal fallback 与 pending interaction owner。
 * OUTPUT: 可由 stopped/error 收口的只读工具执行证据；结构化问答与权限操作统一由 Composer 持有。
 * POS: StructuredContent 中禁止消息上下文重新挂载人工交互选项的工具展示边界。
 */
import type { ReactNode } from "react";

import { isGenerativeUIWidgetToolName } from "@/lib/conversation/generative-ui";
import type {
  PendingPermission,
  PermissionDecisionPayload,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";
import type {
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message/content";

import { GenerativeUIBlock } from "../../../blocks/tool/generative-ui-block";
import { ToolBlock } from "../../../blocks/tool/tool-block";
import type { ToolPermissionRequest } from "../../../blocks/tool/tool-block-types";
import { isRejectedToolResult } from "../../../tool-result-semantic-model";
import type { PendingInteractionOwner } from "../../message-item-projection";
import type { UnresolvedToolStatus } from "./content-renderer-contract";
import {
  resolveToolBlockStatus,
  type StructuredContentProjection,
  type ToolUseProjection,
} from "./content-renderer-model";

interface ContentToolBlockContext {
  canRespondToPermissions: boolean;
  defaultToolDetailsExpanded: boolean;
  onOpenSubagentTask?: (
    toolUseId: string,
    hostAgentId?: string | null,
  ) => void;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingInteractionOwner: PendingInteractionOwner;
  pendingPermission?: PendingPermission;
  permissionReadOnlyReason?: string;
  projection: StructuredContentProjection;
  unresolvedToolStatus?: UnresolvedToolStatus;
  workspaceAgentId?: string | null;
}

interface ContentToolBlockState {
  result?: ToolResultContent;
  toolUse?: ToolUseProjection;
  waitingForPermission: boolean;
}

export function ContentToolBlock({
  block,
  context,
}: {
  block: ToolUseContent;
  context: ContentToolBlockContext;
}): ReactNode {
  const state = resolveContentToolBlockState(
    block,
    context.pendingPermission,
    context.projection,
  );
  const failed = state.result?.is_error
    || (state.result ? isRejectedToolResult(state.result) : false);
  if (isGenerativeUIWidgetToolName(block.name) && !failed) {
    return (
      <GenerativeUIBlock
        complete={Boolean(state.result)}
        toolUse={block}
      />
    );
  }
  return renderStandardToolBlock(block, context, state);
}

function renderStandardToolBlock(
  block: ToolUseContent,
  context: ContentToolBlockContext,
  state: ContentToolBlockState,
) {
  return (
    <div className="min-w-0">
      <ToolBlock
        defaultExpanded={context.defaultToolDetailsExpanded}
        interactionDisabled={!context.canRespondToPermissions}
        interactionDisabledReason={context.permissionReadOnlyReason}
        liveProgress={context.projection.taskProgressByToolUseId.get(block.id) ?? null}
        onOpenSubagentTask={context.onOpenSubagentTask}
        onOpenWorkspaceFile={context.onOpenWorkspaceFile}
        permissionRequest={resolvePermissionRequest(context, state)}
        status={resolveToolBlockStatus(
          state.toolUse,
          state.waitingForPermission,
          context.unresolvedToolStatus,
        )}
        toolResult={state.result}
        toolUse={block}
        workspaceAgentId={context.workspaceAgentId}
      />
    </div>
  );
}

function resolveContentToolBlockState(
  block: ToolUseContent,
  pendingPermission: PendingPermission | undefined,
  projection: StructuredContentProjection,
): ContentToolBlockState {
  const toolUse = projection.toolUseById.get(block.id);
  return {
    result: toolUse?.result,
    toolUse,
    waitingForPermission: Boolean(pendingPermission) && !toolUse?.result,
  };
}

function resolvePermissionRequest(
  context: ContentToolBlockContext,
  state: ContentToolBlockState,
): ToolPermissionRequest | undefined {
  if (
    context.pendingInteractionOwner !== "content"
    || !context.pendingPermission
    || !state.waitingForPermission
  ) {
    return undefined;
  }
  return createPermissionRequest(
    context.pendingPermission,
    context.onPermissionResponse,
  );
}

function createPermissionRequest(
  permission: PendingPermission,
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean,
): ToolPermissionRequest {
  const respond = (
    decision: PermissionDecisionPayload["decision"],
    updatedPermissions?: PermissionUpdate[],
  ) => {
    onPermissionResponse?.({
      decision,
      request_id: permission.request_id,
      updated_permissions: updatedPermissions,
    });
  };

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
