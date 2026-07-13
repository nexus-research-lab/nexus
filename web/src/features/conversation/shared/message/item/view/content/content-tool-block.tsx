import type { ReactNode } from "react";

import type { UserQuestionAnswer } from "@/types/conversation/interaction/ask-user-question";
import type {
  PendingPermission,
  PermissionDecisionPayload,
  PermissionUpdate,
} from "@/types/conversation/interaction/permission";
import type {
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message/content";

import { ASK_USER_QUESTION_TOOL_NAME } from "../../../message-tool-names";
import { AskUserQuestionBlock } from "../../../blocks/question/ask-user-question-block";
import { ToolBlock } from "../../../blocks/tool/tool-block";
import type { ToolPermissionRequest } from "../../../blocks/tool/tool-block-types";
import {
  resolveToolBlockStatus,
  type StructuredContentProjection,
  type ToolUseProjection,
} from "./content-renderer-model";

interface ContentToolBlockContext {
  canRespondToPermissions: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingPermission?: PendingPermission;
  permissionReadOnlyReason?: string;
  projection: StructuredContentProjection;
  workspaceAgentId?: string | null;
}

interface ContentToolBlockState {
  result?: ToolResultContent;
  toolUse?: ToolUseProjection;
  waitingForPermission: boolean;
}

type QuestionSubmit = (
  toolUseId: string,
  answers: UserQuestionAnswer[],
) => boolean;

const DISABLED_QUESTION_SUBMIT: QuestionSubmit = () => false;

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
  if (block.name === ASK_USER_QUESTION_TOOL_NAME) {
    return renderQuestionToolBlock(block, context, state);
  }
  return renderStandardToolBlock(block, context, state);
}

function renderQuestionToolBlock(
  block: ToolUseContent,
  context: ContentToolBlockContext,
  state: ContentToolBlockState,
) {
  return (
    <AskUserQuestionBlock
      initialSubmitted={isSuccessfulResult(state.result)}
      interactionDisabled={!context.canRespondToPermissions}
      isReady={state.waitingForPermission}
      onSubmit={createQuestionSubmit(context)}
      toolResult={state.result}
      toolUse={block}
    />
  );
}

function renderStandardToolBlock(
  block: ToolUseContent,
  context: ContentToolBlockContext,
  state: ContentToolBlockState,
) {
  return (
    <div className="min-w-0">
      <ToolBlock
        interactionDisabled={!context.canRespondToPermissions}
        interactionDisabledReason={context.permissionReadOnlyReason}
        liveProgress={context.projection.taskProgressByToolUseId.get(block.id) ?? null}
        onOpenWorkspaceFile={context.onOpenWorkspaceFile}
        permissionRequest={resolvePermissionRequest(context, state)}
        status={resolveToolBlockStatus(state.toolUse, state.waitingForPermission)}
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

function createQuestionSubmit(
  context: ContentToolBlockContext,
): QuestionSubmit {
  const { onPermissionResponse, pendingPermission } = context;
  if (!pendingPermission || !onPermissionResponse) {
    return DISABLED_QUESTION_SUBMIT;
  }
  return (_toolUseId, answers) => onPermissionResponse({
    decision: "allow",
    request_id: pendingPermission.request_id,
    user_answers: answers,
  });
}

function isSuccessfulResult(result: ToolResultContent | undefined): boolean {
  return Boolean(result && !result.is_error);
}

function resolvePermissionRequest(
  context: ContentToolBlockContext,
  state: ContentToolBlockState,
): ToolPermissionRequest | undefined {
  if (!context.pendingPermission || !state.waitingForPermission) {
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
