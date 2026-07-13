import { useCallback, useRef, useState } from "react";

export interface ScopedMemoryScope {
  key: string;
}

export interface ScopedMemoryState {
  scopeKey: string;
}

export interface ScopedMemoryScopeRef<Scope extends ScopedMemoryScope> {
  current: Scope;
}

export type ScopedMemoryCommit<State extends ScopedMemoryState> = (
  expectedScopeKey: string,
  update: (current: State) => State,
) => void;

/**
 * Memory 请求可能跨越 Agent 或文档切换；提交入口同时校验渲染作用域和更新时作用域，
 * 避免旧异步结果在 React 排队更新期间写入新页面。
 */
export function useScopedMemoryState<
  Scope extends ScopedMemoryScope,
  State extends ScopedMemoryState,
>(
  scope: Scope,
  createState: (scopeKey: string) => State,
): {
  commit: ScopedMemoryCommit<State>;
  scopeRef: ScopedMemoryScopeRef<Scope>;
  state: State;
} {
  const scopeRef = useRef(scope);
  scopeRef.current = scope;
  const createStateRef = useRef(createState);
  createStateRef.current = createState;
  const [storedState, setStoredState] = useState<State>(() => createState(scope.key));
  const state = storedState.scopeKey === scope.key
    ? storedState
    : createState(scope.key);

  const commit = useCallback<ScopedMemoryCommit<State>>((expectedScopeKey, update) => {
    if (scopeRef.current.key !== expectedScopeKey) {
      return;
    }
    setStoredState((current) => {
      if (scopeRef.current.key !== expectedScopeKey) {
        return current;
      }
      const scopedState = current.scopeKey === expectedScopeKey
        ? current
        : createStateRef.current(expectedScopeKey);
      return update(scopedState);
    });
  }, []);

  return { commit, scopeRef, state };
}
