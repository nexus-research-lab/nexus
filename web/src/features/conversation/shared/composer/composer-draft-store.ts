/**
 * INPUT: Room/DM Session 草稿作用域、完整 Composer 草稿更新与提交时修订号。
 * OUTPUT: 按 Session 保留的文字、附件、模式、Goal 负责人和 Mention 目标快照。
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

type ComposerDraftUpdate = (
  current: ComposerDraftContent,
) => ComposerDraftContent;

interface ComposerDraftStoreState {
  draft_revision: number;
  drafts_by_scope: Record<string, ComposerDraftSnapshot>;
  clear_composer_draft_if_revision: (
    scopeKey: string,
    expectedRevision: number,
  ) => boolean;
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
    clear_composer_draft_if_revision: (scopeKey, expectedRevision) => {
      const normalizedScopeKey = normalizeDraftScopeKey(scopeKey);
      if (!normalizedScopeKey) {
        return false;
      }
      let cleared = false;
      set((state) => {
        const current = state.drafts_by_scope[normalizedScopeKey];
        if (!current || current.revision !== expectedRevision) {
          return state;
        }
        const drafts = { ...state.drafts_by_scope };
        delete drafts[normalizedScopeKey];
        cleared = true;
        return { drafts_by_scope: drafts };
      });
      return cleared;
    },
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
