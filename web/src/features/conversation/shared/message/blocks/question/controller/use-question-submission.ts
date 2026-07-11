import {
  useCallback,
  useEffect,
  useRef,
  type MutableRefObject,
} from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type { UserQuestionAnswer } from "@/types/conversation/interaction/ask-user-question";

import {
  buildQuestionAnswers,
  type QuestionDraft,
} from "../ask-user-question-model";

interface SubmissionToken {
  scopeKey: string;
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
  const activeScopeRef = useRef(scopeKey);
  activeScopeRef.current = scopeKey;
  const activeSubmissionRef = useRef<SubmissionToken | null>(null);
  const [isSubmitting, setIsSubmitting] = useResettableState(false, scopeKey);
  const submitEnabled = [submissionReady, !isSubmitting].every(Boolean);

  useEffect(() => () => {
    if (activeScopeRef.current === scopeKey) {
      activeScopeRef.current = "";
    }
  }, [scopeKey]);

  const submit = useCallback(async () => {
    await runQuestionSubmission({
      activeScopeRef,
      activeSubmissionRef,
      draft,
      onAccepted,
      onCollapse,
      onSubmit,
      scopeKey,
      setIsSubmitting,
      submitEnabled,
      toolUseId,
    });
  }, [
    draft,
    onAccepted,
    onCollapse,
    onSubmit,
    scopeKey,
    setIsSubmitting,
    submitEnabled,
    toolUseId,
  ]);

  return { isSubmitting, submit, submitEnabled };
}

interface QuestionSubmissionTransaction {
  activeScopeRef: MutableRefObject<string>;
  activeSubmissionRef: MutableRefObject<SubmissionToken | null>;
  draft: QuestionDraft;
  onAccepted: () => void;
  onCollapse: () => void;
  onSubmit: UseQuestionSubmissionOptions["onSubmit"];
  scopeKey: string;
  setIsSubmitting: (value: boolean) => void;
  submitEnabled: boolean;
  toolUseId: string;
}

async function runQuestionSubmission(
  transaction: QuestionSubmissionTransaction,
): Promise<void> {
  const token = beginSubmission(transaction);
  if (!token) {
    return;
  }
  try {
    const accepted = await transaction.onSubmit(
      transaction.toolUseId,
      buildQuestionAnswers(transaction.draft),
    );
    applyAcceptedSubmission(transaction, token, accepted);
  } finally {
    finishSubmission(transaction, token);
  }
}

function beginSubmission(
  transaction: QuestionSubmissionTransaction,
): SubmissionToken | null {
  const duplicate =
    transaction.activeSubmissionRef.current?.scopeKey === transaction.scopeKey;
  if (!transaction.submitEnabled || duplicate) {
    return null;
  }
  const token = { scopeKey: transaction.scopeKey };
  transaction.activeSubmissionRef.current = token;
  transaction.setIsSubmitting(true);
  return token;
}

function applyAcceptedSubmission(
  transaction: QuestionSubmissionTransaction,
  token: SubmissionToken,
  accepted: boolean,
): void {
  const currentScope = transaction.activeScopeRef.current === token.scopeKey;
  if (!accepted || !currentScope) {
    return;
  }
  transaction.onAccepted();
  transaction.onCollapse();
}

function finishSubmission(
  transaction: QuestionSubmissionTransaction,
  token: SubmissionToken,
): void {
  if (transaction.activeSubmissionRef.current === token) {
    transaction.activeSubmissionRef.current = null;
  }
  if (transaction.activeScopeRef.current === token.scopeKey) {
    transaction.setIsSubmitting(false);
  }
}
