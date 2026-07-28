/**
 * INPUT: 要求用户提交结构化回答的 runtime 交互，以及可选的原始工具块。
 * OUTPUT: 可直接回答或拒绝的人工介入交互面。
 * POS: 独立 pending 列表与已匹配消息工具块共用的结构化输入适配器。
 */
import { AskUserQuestionBlock } from "@/features/conversation/shared/message/blocks/question/ask-user-question-block";
import { useI18n } from "@/shared/i18n/i18n-context";
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
  readOnlyReason?: string;
  toolUse?: ToolUseContent;
}

export function PendingHumanQuestion({
  canRespond,
  onResponse,
  permission,
  readOnlyReason,
  toolUse,
}: PendingHumanQuestionProps) {
  const { t } = useI18n();
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
    <div className="space-y-2">
      <AskUserQuestionBlock
        interactionDisabled={interactionDisabled}
        isReady={!interactionDisabled}
        onSubmit={submitQuestion}
        toolUse={toolUse ?? {
          id: pendingQuestionToolUseId(permission),
          input: permission.tool_input,
          name: permission.tool_name,
          type: "tool_use",
        }}
      />
      {interactionDisabled ? (
        readOnlyReason ? (
          <div className="text-xs text-(--text-soft)">
            {readOnlyReason}
          </div>
        ) : null
      ) : (
        <div className="flex justify-end">
          <button
            className="rounded-md border border-(--divider-subtle-color) bg-transparent px-2.5 py-1.5 text-xs font-medium text-(--text-default) transition-colors hover:bg-(--interaction-hover-background)"
            onClick={(event) => {
              event.stopPropagation();
              denyQuestion();
            }}
            type="button"
          >
            {t("room.permission_deny")}
          </button>
        </div>
      )}
    </div>
  );
}

function pendingQuestionToolUseId(permission: PendingPermission): string {
  return permission.tool_use_id?.trim()
    || `pending_${permission.request_id}`;
}
