import { useCallback, useEffect, useRef, useState } from "react";

import type { AgentNameValidationResult } from "@/types/agent/agent";

interface UseAgentNameValidationOptions {
  fallbackError: string;
  hasTitleChanged: boolean;
  isActive: boolean;
  onValidateName?: (name: string) => Promise<AgentNameValidationResult>;
  scopeKey: string;
  title: string;
}

interface ValidationScope {
  key: string;
  validator?: (name: string) => Promise<AgentNameValidationResult>;
}

interface ValidationState {
  pendingName: string | null;
  result: AgentNameValidationResult | null;
  scopeKey: string;
}

interface PendingValidation {
  name: string;
  promise: Promise<AgentNameValidationResult | null>;
  requestSequence: number;
  scopeKey: string;
}

export function useAgentNameValidation({
  fallbackError,
  hasTitleChanged,
  isActive,
  onValidateName,
  scopeKey,
  title,
}: UseAgentNameValidationOptions) {
  const trimmedTitle = title.trim();
  const scopeRef = useRef<ValidationScope>({ key: scopeKey, validator: onValidateName });
  scopeRef.current = { key: scopeKey, validator: onValidateName };
  const requestSequenceRef = useRef(0);
  const pendingValidationRef = useRef<PendingValidation | null>(null);
  const debounceTimerRef = useRef<number | null>(null);
  const [storedState, setStoredState] = useState<ValidationState>(() =>
    createValidationState(scopeKey),
  );
  const state = storedState.scopeKey === scopeKey
    ? storedState
    : createValidationState(scopeKey);

  const commit = useCallback((expectedScopeKey: string, update: (
    current: ValidationState,
  ) => ValidationState) => {
    if (scopeRef.current.key !== expectedScopeKey) {
      return;
    }
    setStoredState((current) => {
      if (scopeRef.current.key !== expectedScopeKey) {
        return current;
      }
      return update(
        current.scopeKey === expectedScopeKey
          ? current
          : createValidationState(expectedScopeKey),
      );
    });
  }, []);

  const clearDebounce = useCallback(() => {
    if (debounceTimerRef.current !== null) {
      window.clearTimeout(debounceTimerRef.current);
      debounceTimerRef.current = null;
    }
  }, []);

  const validateNow = useCallback((
    name: string,
  ): Promise<AgentNameValidationResult | null> => {
    clearDebounce();
    const scope = scopeRef.current;
    if (scope.key !== scopeKey || !scope.validator) {
      return Promise.resolve(null);
    }
    const pendingValidation = pendingValidationRef.current;
    if (
      pendingValidation?.scopeKey === scope.key
      && pendingValidation.name === name
    ) {
      return pendingValidation.promise;
    }
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    commit(scope.key, (current) => ({ ...current, pendingName: name }));

    const request = scope.validator(name)
      .catch((error: unknown) => buildFailedValidation(name, error, fallbackError))
      .then((result) => {
        if (
          scopeRef.current.key === scope.key
          && requestSequenceRef.current === requestSequence
        ) {
          commit(scope.key, (current) => ({
            ...current,
            pendingName: null,
            result,
          }));
        }
        return result;
      })
      .finally(() => {
        if (pendingValidationRef.current?.requestSequence === requestSequence) {
          pendingValidationRef.current = null;
        }
      });
    pendingValidationRef.current = {
      name,
      promise: request,
      requestSequence,
      scopeKey: scope.key,
    };
    return request;
  }, [clearDebounce, commit, fallbackError, scopeKey]);

  useEffect(() => {
    requestSequenceRef.current += 1;
    const shouldValidate = Boolean(
      isActive && onValidateName && trimmedTitle && hasTitleChanged,
    );
    if (!shouldValidate) {
      commit(scopeKey, (current) => ({
        ...current,
        pendingName: null,
        result: null,
      }));
      return undefined;
    }
    clearDebounce();
    debounceTimerRef.current = window.setTimeout(() => {
      debounceTimerRef.current = null;
      void validateNow(trimmedTitle);
    }, 300);
    return () => {
      clearDebounce();
      requestSequenceRef.current += 1;
    };
  }, [
    clearDebounce,
    commit,
    hasTitleChanged,
    isActive,
    onValidateName,
    scopeKey,
    trimmedTitle,
    validateNow,
  ]);

  const currentResult = state.result?.name === trimmedTitle ? state.result : null;
  return {
    isValidating: state.pendingName === trimmedTitle,
    result: currentResult,
    validateNow,
  };
}

function createValidationState(scopeKey: string): ValidationState {
  return { pendingName: null, result: null, scopeKey };
}

function buildFailedValidation(
  name: string,
  error: unknown,
  fallbackError: string,
): AgentNameValidationResult {
  return {
    name,
    normalized_name: name,
    is_valid: false,
    is_available: false,
    reason: error instanceof Error ? error.message : fallbackError,
    workspace_path: null,
  };
}
