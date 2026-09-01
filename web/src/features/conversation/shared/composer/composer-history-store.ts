/**
 * INPUT: Room/DM 逻辑聊天作用域、成功投递的 Composer 正文与 owner reset。
 * OUTPUT: 每个客户端本地持久化、按聊天隔离、跨 owner 清空且有界的输入历史。
 * POS: Composer 输入历史的浏览器/WebView 真相源；不参与服务端或跨设备同步。
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";

import { MAX_COMPOSER_INPUT_LENGTH } from "./composer-model";

export const MAX_COMPOSER_HISTORY_ITEMS = 50;
const MAX_PERSISTED_COMPOSER_HISTORY_ITEMS = 100;

interface PersistedComposerHistoryState {
  items_by_scope?: unknown;
}

interface ComposerHistoryStoreState {
  items_by_scope: Record<string, string[]>;
  record_composer_history: (scopeKey: string, value: string) => void;
}

export const useComposerHistoryStore = create<ComposerHistoryStoreState>()(
  persist(
    (set) => ({
      items_by_scope: {},
      record_composer_history: (scopeKey, value) => set((state) => {
        const normalizedScopeKey = normalizeComposerHistoryScopeKey(scopeKey);
        const normalizedValue = normalizeComposerHistoryValue(value);
        if (!normalizedScopeKey || !normalizedValue) {
          return state;
        }
        const itemsByScope = { ...state.items_by_scope };
        const currentItems = itemsByScope[normalizedScopeKey] ?? [];
        delete itemsByScope[normalizedScopeKey];
        itemsByScope[normalizedScopeKey] = [
          normalizedValue,
          ...currentItems,
        ].slice(0, MAX_COMPOSER_HISTORY_ITEMS);
        return {
          items_by_scope: limitPersistedComposerHistory(itemsByScope),
        };
      }),
    }),
    {
      name: "nexus-composer-history",
      partialize: (state) => ({
        items_by_scope: state.items_by_scope,
      }),
      storage: createBrowserJsonStorage(),
      version: 1,
      migrate: (persistedState: unknown): PersistedComposerHistoryState => {
        const state = (persistedState ?? {}) as PersistedComposerHistoryState;
        return {
          items_by_scope: normalizePersistedComposerHistory(
            state.items_by_scope,
          ),
        };
      },
    },
  ),
);

/** 登出或 owner 切换时删除本机保存的上一账号发送历史。 */
export function resetComposerHistoryOwnerScope(): void {
  useComposerHistoryStore.setState({ items_by_scope: {} });
}

function normalizeComposerHistoryScopeKey(scopeKey: string): string {
  return scopeKey.trim();
}

function normalizeComposerHistoryValue(value: unknown): string {
  if (typeof value !== "string") {
    return "";
  }
  return value.trim().slice(0, MAX_COMPOSER_INPUT_LENGTH);
}

function normalizePersistedComposerHistory(
  value: unknown,
): Record<string, string[]> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  const itemsByScope: Record<string, string[]> = {};
  for (const [scopeKey, items] of Object.entries(value)) {
    const normalizedScopeKey = normalizeComposerHistoryScopeKey(scopeKey);
    if (!normalizedScopeKey || !Array.isArray(items)) {
      continue;
    }
    const normalizedItems = items
      .map(normalizeComposerHistoryValue)
      .filter(Boolean)
      .slice(0, MAX_COMPOSER_HISTORY_ITEMS);
    if (normalizedItems.length > 0) {
      itemsByScope[normalizedScopeKey] = normalizedItems;
    }
  }
  return limitPersistedComposerHistory(itemsByScope);
}

function limitPersistedComposerHistory(
  itemsByScope: Record<string, string[]>,
): Record<string, string[]> {
  let remainingItemCount = MAX_PERSISTED_COMPOSER_HISTORY_ITEMS;
  const retainedScopes: Array<[string, string[]]> = [];
  for (const [scopeKey, items] of Object.entries(itemsByScope).reverse()) {
    if (remainingItemCount <= 0) {
      break;
    }
    const retainedItems = items.slice(
      0,
      Math.min(remainingItemCount, MAX_COMPOSER_HISTORY_ITEMS),
    );
    if (retainedItems.length === 0) {
      continue;
    }
    retainedScopes.push([scopeKey, retainedItems]);
    remainingItemCount -= retainedItems.length;
  }
  return Object.fromEntries(retainedScopes.reverse());
}
