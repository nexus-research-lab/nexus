/**
 * INPUT: 要求用户提交结构化回答的 runtime pending 交互与当前响应能力。
 * OUTPUT: 以 request_id 保持表单与草稿身份稳定、可回答或拒绝的交互面。
 * POS: Composer 唯一 pending 问答适配器；不接受历史工具块或以 tool_use_id 覆盖请求身份。
 */
import { AskUserQuestionBlock } from "@/features/conversation/shared/message/blocks/question/ask-user-question-block";
import type { UserQuestionAnswer } from "@/types/conversation/interaction/ask-user-question";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

interface PendingHumanQuestionProps {
  canRespond: boolean;
  onResponse?: (payload: PermissionDecisionPayload) => boolean;
  permission: PendingPermission;
}

export function PendingHumanQuestion({
  canRespond,
  onResponse,
  permission,
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
      toolUse={{
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
