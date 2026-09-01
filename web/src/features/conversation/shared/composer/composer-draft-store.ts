/**
 * INPUT: Room/DM Session 草稿作用域、完整 Composer 草稿更新与消息/Goal 本地派发事务。
 * OUTPUT: 按 Session 保留的文字、附件、模式、Goal 负责人、Mention 目标、Goal 提交/恢复回执、失败错误与 owner reset。
 * POS: Composer 用户输入草稿的 owner-scoped 客户端内存真相源；不持久化瞬时 UI 或浏览器刷新。
 */

import { create } from "zustand";

import type { ComposerLocalAttachment } from "./attachments/composer-local-attachment-model";
import type { ComposerInputMode } from "./composer-model";
import type { ComposerGoalBaseline } from "./composer-goal-observation";

export interface ComposerDraftContent {
  attachments: ComposerLocalAttachment[];
  goalLeadAgentId: string | null;
  input: string;
  inputMode: ComposerInputMode;
  selectedTargetIDs: string[];
}

export interface ComposerDraftSnapshot extends ComposerDraftContent {
  revision: number;
}

export interface ComposerGoalSubmission {
  baselineGoal: ComposerGoalBaseline | null;
  scopeKey: string;
  submissionId: number;
  submittedDraft: ComposerDraftSnapshot;
}

export interface ComposerGoalConfirmationIdentity {
  clientMessageId: string;
  clientRequestId: string;
  sessionKey: string;
}

export type ComposerGoalSubmissionPhase = "confirming" | "submitting";

export interface ComposerGoalSubmissionState
  extends ComposerGoalSubmission {
  confirmationIdentity: ComposerGoalConfirmationIdentity | null;
  phase: ComposerGoalSubmissionPhase;
}

export interface ComposerGoalRecovery extends ComposerGoalSubmission {
  confirmationIdentity: ComposerGoalConfirmationIdentity;
  restoredDraftRevision: number;
}

type ComposerDraftUpdate = (
  current: ComposerDraftContent,
) => ComposerDraftContent;

interface ComposerDraftStoreState {
  draft_revision: number;
  drafts_by_scope: Record<string, ComposerDraftSnapshot>;
  goal_error_by_scope: Record<string, string>;
  goal_recovery_by_scope: Record<string, ComposerGoalRecovery>;
  goal_submission_revision: number;
  goal_submission_by_scope: Record<string, ComposerGoalSubmissionState>;
  begin_goal_submission: (
    scopeKey: string,
    expectedRevision: number,
    baselineGoal?: ComposerGoalBaseline | null,
  ) => ComposerGoalSubmission | null;
  claim_composer_draft_for_submission: (
    scopeKey: string,
    expectedRevision: number,
  ) => ComposerDraftSnapshot | null;
  restore_composer_draft_after_failed_submission: (
    scopeKey: string,
    submittedDraft: ComposerDraftSnapshot,
  ) => boolean;
  complete_goal_submission: (submission: ComposerGoalSubmission) => boolean;
  complete_goal_recovery: (recovery: ComposerGoalRecovery) => boolean;
  mark_goal_submission_confirming: (
    submission: ComposerGoalSubmission,
    confirmationIdentity?: ComposerGoalConfirmationIdentity | null,
  ) => boolean;
  fail_goal_submission: (
    submission: ComposerGoalSubmission,
    errorMessage: string,
    confirmationIdentity?: ComposerGoalConfirmationIdentity | null,
  ) => boolean;
  set_goal_error: (scopeKey: string, errorMessage: string | null) => void;
  update_composer_draft: (
    scopeKey: string,
    update: ComposerDraftUpdate,
  ) => void;
}

export const EMPTY_COMPOSER_DRAFT: ComposerDraftSnapshot = {
  attachments: [],
  goalLeadAgentId: null,
  input: "",
  inputMode: "message",
  revision: 0,
  selectedTargetIDs: [],
};

const invalidatedOwnerDrafts = new WeakSet<ComposerDraftSnapshot>();

function hasSameItems<T>(left: T[], right: T[]): boolean {
  return left.length === right.length
    && left.every((item, index) => item === right[index]);
}

function hasSameDraftContent(
  current: ComposerDraftContent,
  next: ComposerDraftContent,
): boolean {
  return current.goalLeadAgentId === next.goalLeadAgentId
    && current.input === next.input
    && current.inputMode === next.inputMode
    && hasSameItems(current.attachments, next.attachments)
    && hasSameItems(current.selectedTargetIDs, next.selectedTargetIDs);
}

function normalizeDraftScopeKey(scopeKey: string): string {
  return scopeKey.trim();
}

export const useComposerDraftStore = create<ComposerDraftStoreState>()(
  (set) => ({
    draft_revision: 0,
    drafts_by_scope: {},
    goal_error_by_scope: {},
    goal_recovery_by_scope: {},
    goal_submission_revision: 0,
    goal_submission_by_scope: {},
    begin_goal_submission: (scopeKey, expectedRevision, baselineGoal = null) => {
      const normalizedScopeKey = normalizeDraftScopeKey(scopeKey);
      if (!normalizedScopeKey) {
        return null;
      }
      let submission: ComposerGoalSubmission | null = null;
      set((state) => {
        if (state.goal_submission_by_scope[normalizedScopeKey]) {
          return state;
        }
        const current = state.drafts_by_scope[normalizedScopeKey];
        if (
          (current && current.revision !== expectedRevision)
          || (!current && expectedRevision !== EMPTY_COMPOSER_DRAFT.revision)
        ) {
          return state;
        }
        const submittedDraft = current ?? EMPTY_COMPOSER_DRAFT;
        const submissionId = state.goal_submission_revision + 1;
        const drafts = { ...state.drafts_by_scope };
        const errors = { ...state.goal_error_by_scope };
        const recoveries = { ...state.goal_recovery_by_scope };
        if (current) {
          delete drafts[normalizedScopeKey];
        }
        delete errors[normalizedScopeKey];
        delete recoveries[normalizedScopeKey];
        submission = {
          baselineGoal,
          scopeKey: normalizedScopeKey,
          submissionId,
          submittedDraft: cloneDraftSnapshot(submittedDraft),
        };
        return {
          drafts_by_scope: drafts,
          goal_error_by_scope: errors,
          goal_recovery_by_scope: recoveries,
          goal_submission_revision: submissionId,
          goal_submission_by_scope: {
            ...state.goal_submission_by_scope,
            [normalizedScopeKey]: {
              ...submission,
              confirmationIdentity: null,
              phase: "submitting",
            },
          },
        };
      });
      return submission;
    },
    claim_composer_draft_for_submission: (scopeKey, expectedRevision) => {
      const normalizedScopeKey = normalizeDraftScopeKey(scopeKey);
      if (!normalizedScopeKey) {
        return null;
      }
      let submittedDraft: ComposerDraftSnapshot | null = null;
      set((state) => {
        const current = state.drafts_by_scope[normalizedScopeKey];
        if (!current || current.revision !== expectedRevision) {
          return state;
        }
        const drafts = { ...state.drafts_by_scope };
        delete drafts[normalizedScopeKey];
        submittedDraft = current;
        return { drafts_by_scope: drafts };
      });
      return submittedDraft;
    },
    restore_composer_draft_after_failed_submission: (
      scopeKey,
      submittedDraft,
    ) => {
      if (invalidatedOwnerDrafts.has(submittedDraft)) {
        return false;
      }
      const normalizedScopeKey = normalizeDraftScopeKey(scopeKey);
      if (!normalizedScopeKey) {
        return false;
      }
      let restored = false;
      set((state) => {
        if (state.drafts_by_scope[normalizedScopeKey]) {
          return state;
        }
        const revision = state.draft_revision + 1;
        restored = true;
        return {
          draft_revision: revision,
          drafts_by_scope: {
            ...state.drafts_by_scope,
            [normalizedScopeKey]: {
              attachments: [...submittedDraft.attachments],
              goalLeadAgentId: submittedDraft.goalLeadAgentId,
              input: submittedDraft.input,
              inputMode: submittedDraft.inputMode,
              revision,
              selectedTargetIDs: [...submittedDraft.selectedTargetIDs],
            },
          },
        };
      });
      return restored;
    },
    complete_goal_submission: (submission) => {
      let completed = false;
      set((state) => {
        if (
          state.goal_submission_by_scope[submission.scopeKey]?.submissionId
          !== submission.submissionId
        ) {
          return state;
        }
        const pending = { ...state.goal_submission_by_scope };
        delete pending[submission.scopeKey];
        completed = true;
        return { goal_submission_by_scope: pending };
      });
      return completed;
    },
    complete_goal_recovery: (recovery) => {
      let completed = false;
      set((state) => {
        const currentRecovery = state.goal_recovery_by_scope[recovery.scopeKey];
        if (
          currentRecovery?.submissionId !== recovery.submissionId
          || currentRecovery.confirmationIdentity.clientMessageId
            !== recovery.confirmationIdentity.clientMessageId
          || currentRecovery.confirmationIdentity.clientRequestId
            !== recovery.confirmationIdentity.clientRequestId
          || currentRecovery.confirmationIdentity.sessionKey
            !== recovery.confirmationIdentity.sessionKey
        ) {
          return state;
        }
        const recoveries = { ...state.goal_recovery_by_scope };
        const errors = { ...state.goal_error_by_scope };
        delete recoveries[recovery.scopeKey];
        delete errors[recovery.scopeKey];
        completed = true;
        const currentDraft = state.drafts_by_scope[recovery.scopeKey];
        if (currentDraft?.revision !== recovery.restoredDraftRevision) {
          return {
            goal_error_by_scope: errors,
            goal_recovery_by_scope: recoveries,
          };
        }
        const drafts = { ...state.drafts_by_scope };
        delete drafts[recovery.scopeKey];
        return {
          drafts_by_scope: drafts,
          goal_error_by_scope: errors,
          goal_recovery_by_scope: recoveries,
        };
      });
      return completed;
    },
    mark_goal_submission_confirming: (
      submission,
      confirmationIdentity = null,
    ) => {
      let marked = false;
      set((state) => {
        const current = state.goal_submission_by_scope[submission.scopeKey];
        if (
          current?.submissionId !== submission.submissionId
          || current.phase === "confirming"
        ) {
          return state;
        }
        marked = true;
        return {
          goal_submission_by_scope: {
            ...state.goal_submission_by_scope,
            [submission.scopeKey]: {
              ...current,
              confirmationIdentity,
              phase: "confirming",
            },
          },
        };
      });
      return marked;
    },
    fail_goal_submission: (
      submission,
      errorMessage,
      confirmationIdentity = null,
    ) => {
      let restored = false;
      set((state) => {
        if (
          state.goal_submission_by_scope[submission.scopeKey]?.submissionId
            !== submission.submissionId
        ) {
          return state;
        }
        const pending = { ...state.goal_submission_by_scope };
        delete pending[submission.scopeKey];
        if (state.drafts_by_scope[submission.scopeKey]) {
          return { goal_submission_by_scope: pending };
        }
        const revision = state.draft_revision + 1;
        const normalizedError = errorMessage.trim();
        const errors = { ...state.goal_error_by_scope };
        if (normalizedError) {
          errors[submission.scopeKey] = normalizedError;
        } else {
          delete errors[submission.scopeKey];
        }
        const recoveries = { ...state.goal_recovery_by_scope };
        if (confirmationIdentity) {
          recoveries[submission.scopeKey] = {
            ...submission,
            confirmationIdentity,
            restoredDraftRevision: revision,
          };
        } else {
          delete recoveries[submission.scopeKey];
        }
        restored = true;
        return {
          draft_revision: revision,
          drafts_by_scope: {
            ...state.drafts_by_scope,
            [submission.scopeKey]: {
              ...cloneDraftSnapshot(submission.submittedDraft),
              revision,
            },
          },
          goal_error_by_scope: errors,
          goal_recovery_by_scope: recoveries,
          goal_submission_by_scope: pending,
        };
      });
      return restored;
    },
    set_goal_error: (scopeKey, errorMessage) => set((state) => {
      const normalizedScopeKey = normalizeDraftScopeKey(scopeKey);
      if (!normalizedScopeKey) {
        return state;
      }
      const normalizedError = errorMessage?.trim() ?? "";
      const current = state.goal_error_by_scope[normalizedScopeKey] ?? "";
      if (current === normalizedError) {
        return state;
      }
      const errors = { ...state.goal_error_by_scope };
      if (normalizedError) {
        errors[normalizedScopeKey] = normalizedError;
      } else {
        delete errors[normalizedScopeKey];
      }
      return { goal_error_by_scope: errors };
    }),
    update_composer_draft: (scopeKey, update) => set((state) => {
      const normalizedScopeKey = normalizeDraftScopeKey(scopeKey);
      if (!normalizedScopeKey) {
        return state;
      }
      const current = state.drafts_by_scope[normalizedScopeKey]
        ?? EMPTY_COMPOSER_DRAFT;
      const next = update(current);
      if (hasSameDraftContent(current, next)) {
        return state;
      }
      const revision = state.draft_revision + 1;
      return {
        draft_revision: revision,
        drafts_by_scope: {
          ...state.drafts_by_scope,
          [normalizedScopeKey]: {
            attachments: [...next.attachments],
            goalLeadAgentId: next.goalLeadAgentId,
            input: next.input,
            inputMode: next.inputMode,
            revision,
            selectedTargetIDs: [...next.selectedTargetIDs],
          },
        },
      };
    }),
  }),
);

/** Auth owner 变化时清空草稿与 Goal 回执，并拒绝旧 owner 的迟到恢复。 */
export function resetComposerDraftOwnerScope(): void {
  const state = useComposerDraftStore.getState();
  Object.values(state.drafts_by_scope).forEach((draft) => {
    invalidatedOwnerDrafts.add(draft);
  });
  Object.values(state.goal_submission_by_scope).forEach((submission) => {
    invalidatedOwnerDrafts.add(submission.submittedDraft);
  });
  Object.values(state.goal_recovery_by_scope).forEach((recovery) => {
    invalidatedOwnerDrafts.add(recovery.submittedDraft);
  });
  useComposerDraftStore.setState({
    draft_revision: state.draft_revision + 1,
    drafts_by_scope: {},
    goal_error_by_scope: {},
    goal_recovery_by_scope: {},
    goal_submission_revision: state.goal_submission_revision + 1,
    goal_submission_by_scope: {},
  });
}

function cloneDraftSnapshot(
  draft: ComposerDraftSnapshot,
): ComposerDraftSnapshot {
  return {
    attachments: [...draft.attachments],
    goalLeadAgentId: draft.goalLeadAgentId,
    input: draft.input,
    inputMode: draft.inputMode,
    revision: draft.revision,
    selectedTargetIDs: [...draft.selectedTargetIDs],
  };
}
