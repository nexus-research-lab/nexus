/**
 * INPUT: 当前逻辑聊天作用域、Composer 正文和召回后的焦点恢复动作。
 * OUTPUT: 当前客户端本地输入历史的记录、上下键游标和未发送草稿恢复。
 * POS: 持久化历史 Store 与 Composer 键盘控制器之间的交互适配层。
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import { useComposerHistoryStore } from "./composer-history-store";

interface UseComposerHistoryOptions {
  clearError: () => void;
  input: string;
  onRecall: () => void;
  scopeKey: string;
  setInput: Dispatch<SetStateAction<string>>;
}

interface ComposerHistoryNavigation {
  draft: string;
  index: number;
  scopeKey: string;
}

const EMPTY_COMPOSER_HISTORY_ITEMS: string[] = [];

export function useComposerHistory({
  clearError,
  input,
  onRecall,
  scopeKey,
  setInput,
}: UseComposerHistoryOptions) {
  const normalizedScopeKey = scopeKey.trim();
  const items = useComposerHistoryStore(
    (state) => state.items_by_scope[normalizedScopeKey]
      ?? EMPTY_COMPOSER_HISTORY_ITEMS,
  );
  const recordComposerHistory = useComposerHistoryStore(
    (state) => state.record_composer_history,
  );
  const [navigation, setNavigation] = useState<ComposerHistoryNavigation>(() => ({
    draft: "",
    index: -1,
    scopeKey: normalizedScopeKey,
  }));
  const activeNavigation = useMemo<ComposerHistoryNavigation>(
    () => navigation.scopeKey === normalizedScopeKey
      ? navigation
      : { draft: "", index: -1, scopeKey: normalizedScopeKey },
    [navigation, normalizedScopeKey],
  );

  useEffect(() => {
    setNavigation((current) => current.scopeKey === normalizedScopeKey
      ? current
      : { draft: "", index: -1, scopeKey: normalizedScopeKey });
  }, [normalizedScopeKey]);

  const record = useCallback((value: string) => {
    recordComposerHistory(normalizedScopeKey, value);
    setNavigation((current) => current.scopeKey === normalizedScopeKey
      ? { draft: "", index: -1, scopeKey: normalizedScopeKey }
      : current);
  }, [normalizedScopeKey, recordComposerHistory]);

  const recallPrevious = useCallback(() => {
    if (items.length === 0) {
      return;
    }
    const nextIndex = Math.min(
      activeNavigation.index + 1,
      items.length - 1,
    );
    setNavigation({
      draft: activeNavigation.index < 0 ? input : activeNavigation.draft,
      index: nextIndex,
      scopeKey: normalizedScopeKey,
    });
    setInput(items[nextIndex] ?? "");
    clearError();
    onRecall();
  }, [
    activeNavigation,
    clearError,
    input,
    items,
    normalizedScopeKey,
    onRecall,
    setInput,
  ]);

  const recallNext = useCallback(() => {
    if (activeNavigation.index > 0) {
      const nextIndex = activeNavigation.index - 1;
      setNavigation({
        ...activeNavigation,
        index: nextIndex,
      });
      setInput(items[nextIndex] ?? "");
    } else if (activeNavigation.index === 0) {
      setNavigation({
        draft: "",
        index: -1,
        scopeKey: normalizedScopeKey,
      });
      setInput(activeNavigation.draft);
    } else {
      return;
    }
    clearError();
    onRecall();
  }, [
    activeNavigation,
    clearError,
    items,
    normalizedScopeKey,
    onRecall,
    setInput,
  ]);

  return {
    index: activeNavigation.index,
    itemCount: items.length,
    recallNext,
    recallPrevious,
    record,
  };
}
