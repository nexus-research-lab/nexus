// INPUT: 精确问题作用域、草稿、提交能力和布尔受理结果。
// OUTPUT: 同步防重、独立作用域代次与异步提交/折叠收口。
// POS: 问答提交生命周期；拒绝由调用方处理，不重放命令或推断服务端结果。

import { useCallback, useEffect, useMemo, useRef } from "react";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { UserQuestionAnswer } from "@/types/conversation/interaction/ask-user-question";
import { buildQuestionAnswers, type QuestionDraft } from "../ask-user-question-model";

interface SubmissionOwner {
  scopeKey: string;
  active: boolean;
}

interface UseQuestionSubmissionOptions {
  draft: QuestionDraft;
  onAccepted: () => void;
  onCollapse: () => void;
  onSubmit: (
    toolUseId: string,
    answers: UserQuestionAnswer[],
  ) => boolean | Promise<boolean>;
  scopeKey: string;
  submissionReady: boolean;
  toolUseId: string;
}

export function useQuestionSubmission({
  draft,
  onAccepted,
  onCollapse,
  onSubmit,
  scopeKey,
  submissionReady,
  toolUseId,
}: UseQuestionSubmissionOptions) {
  // 同一个字符串作用域重新进入时必须取得新身份，不能让旧 Promise 认领新表单。
  const owner = useMemo<SubmissionOwner>(() => ({ scopeKey, active: false }), [scopeKey]);
  const pendingOwnerRef = useRef<SubmissionOwner | null>(null);
  const [isSubmitting, setIsSubmitting] = useResettableState(false, owner);
  const submitEnabled = submissionReady && !isSubmitting;

  useEffect(() => {
    owner.active = true;
    return () => { owner.active = false; };
  }, [owner]);

  const submit = useCallback(async () => {
    if (!owner.active || !submitEnabled || pendingOwnerRef.current === owner) return;
    pendingOwnerRef.current = owner;
    setIsSubmitting(true);
    try {
      const accepted = await onSubmit(toolUseId, buildQuestionAnswers(draft));
      if (accepted && owner.active && pendingOwnerRef.current === owner) {
        onAccepted();
        onCollapse();
      }
    } finally {
      if (pendingOwnerRef.current === owner) pendingOwnerRef.current = null;
      if (owner.active) setIsSubmitting(false);
    }
  }, [draft, onAccepted, onCollapse, onSubmit, owner, setIsSubmitting, submitEnabled, toolUseId]);

  return { isSubmitting, submit, submitEnabled };
}
