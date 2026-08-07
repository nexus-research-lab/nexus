/**
 * INPUT: 要求用户提交结构化回答的 runtime 交互，以及可选的原始工具块。
 * OUTPUT: 以 request_id 保持表单与草稿身份稳定、可回答或拒绝的交互面。
 * POS: Composer 替换面与只读消息工具块共用的中立结构化输入适配器；后到 tool_use_id 只补上下文。
 */
import { AskUserQuestionBlock } from "@/features/conversation/shared/message/blocks/question/ask-user-question-block";
import type { UserQuestionAnswer } from "@/types/conversation/interaction/ask-user-question";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";
import type { ToolUseContent } from "@/types/conversation/message/content";

interface PendingHumanQuestionProps {
  canRespond: boolean;
  onResponse?: (payload: PermissionDecisionPayload) => boolean;
  permission: PendingPermission;
  toolUse?: ToolUseContent;
}

export function PendingHumanQuestion({
  canRespond,
  onResponse,
  permission,
  toolUse,
}: PendingHumanQuestionProps) {
  const interactionDisabled = !canRespond || !onResponse;
  const submitQuestion = (
    _toolUseId: string,
    answers: UserQuestionAnswer[],
  ) => onResponse?.({
    decision: "allow",
    request_id: permission.request_id,
    user_answers: answers,
  }) ?? false;
  const denyQuestion = () => onResponse?.({
    decision: "deny",
    request_id: permission.request_id,
  });

  return (
    <AskUserQuestionBlock
      interactionDisabled={interactionDisabled}
      isReady={!interactionDisabled}
      onDeny={onResponse ? denyQuestion : undefined}
      onSubmit={submitQuestion}
      toolUse={toolUse ?? {
        id: pendingQuestionToolUseId(permission),
        input: permission.tool_input,
        name: permission.tool_name,
        type: "tool_use",
      }}
    />
  );
}

function pendingQuestionToolUseId(permission: PendingPermission): string {
  // request_id 是人工介入生命周期的稳定身份；后到 tool_use_id
  // 只补充消息上下文，不能重置已输入的问答草稿。
  return `pending_${permission.request_id}`;
}
