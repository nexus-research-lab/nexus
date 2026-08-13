/**
 * INPUT: Room/DM Session 草稿作用域、完整 Composer 草稿更新与消息/Goal 本地派发事务。
 * OUTPUT: 按 Session 保留的文字、附件、模式、Goal 负责人、Mention 目标、Goal 提交态与失败错误，并保护迟到结果不覆盖新输入。
 * POS: Composer 用户输入草稿的客户端内存真相源；不持久化瞬时 UI 或浏览器刷新。
 */

import { create } from "zustand";

import type { ComposerLocalAttachment } from "./attachments/composer-local-attachment-model";
import type { ComposerInputMode } from "./composer-model";

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
  scopeKey: string;
  submissionId: number;
  submittedDraft: ComposerDraftSnapshot;
}

type ComposerDraftUpdate = (
  current: ComposerDraftContent,
) => ComposerDraftContent;

interface ComposerDraftStoreState {
  draft_revision: number;
  drafts_by_scope: Record<string, ComposerDraftSnapshot>;
  goal_error_by_scope: Record<string, string>;
  goal_submission_revision: number;
  goal_submission_by_scope: Record<string, number>;
  begin_goal_submission: (
    scopeKey: string,
    expectedRevision: number,
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
  fail_goal_submission: (
    submission: ComposerGoalSubmission,
    errorMessage: string,
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
    goal_submission_revision: 0,
    goal_submission_by_scope: {},
    begin_goal_submission: (scopeKey, expectedRevision) => {
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
        if (current) {
          delete drafts[normalizedScopeKey];
        }
        delete errors[normalizedScopeKey];
        submission = {
          scopeKey: normalizedScopeKey,
          submissionId,
          submittedDraft: cloneDraftSnapshot(submittedDraft),
        };
        return {
          drafts_by_scope: drafts,
          goal_error_by_scope: errors,
          goal_submission_revision: submissionId,
          goal_submission_by_scope: {
            ...state.goal_submission_by_scope,
            [normalizedScopeKey]: submissionId,
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
          state.goal_submission_by_scope[submission.scopeKey]
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
    fail_goal_submission: (submission, errorMessage) => {
      let restored = false;
      set((state) => {
        if (
          state.goal_submission_by_scope[submission.scopeKey]
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
