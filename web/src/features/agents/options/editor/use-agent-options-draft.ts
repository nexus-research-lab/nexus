import { useCallback, useRef, useState } from "react";

import {
  buildAgentOptionsDraftKey,
  reconcileAgentOptionsDraft,
  type AgentOptionsDraft,
} from "./agent-options-draft";

type AgentOptionsDraftField = keyof AgentOptionsDraft;
type ToolListKind = "allowed" | "disallowed";

const TOOL_FIELD_BY_KIND: Readonly<Record<ToolListKind, "allowedTools" | "disallowedTools">> = {
  allowed: "allowedTools",
  disallowed: "disallowedTools",
};

interface AgentOptionsDraftSourceState {
  editorScopeKey: string;
  initialDraft: AgentOptionsDraft;
  sourceScopeKey: string;
}

interface UseAgentOptionsDraftOptions extends AgentOptionsDraftSourceState {
  onChange: () => void;
}

interface AgentOptionsDraftState {
  draft: AgentOptionsDraft;
  editorScopeKey: string;
  revision: number;
  sourceDraft: AgentOptionsDraft;
  sourceScopeKey: string;
}

export function useAgentOptionsDraft({
  editorScopeKey,
  initialDraft,
  onChange,
  sourceScopeKey,
}: UseAgentOptionsDraftOptions) {
  const [storedState, setStoredState] = useState<AgentOptionsDraftState>(() =>
    createDraftState(editorScopeKey, initialDraft, sourceScopeKey),
  );
  const state = reconcileDraftState(storedState, {
    editorScopeKey,
    initialDraft,
    sourceScopeKey,
  });
  if (state !== storedState) {
    setStoredState(state);
  }
  const revisionRef = useRef(0);
  const revisionScopeKeyRef = useRef(editorScopeKey);
  if (revisionScopeKeyRef.current !== editorScopeKey) {
    revisionScopeKeyRef.current = editorScopeKey;
    revisionRef.current = 0;
  }

  const updateField = useCallback(<Field extends AgentOptionsDraftField>(
    field: Field,
    value: AgentOptionsDraft[Field],
  ) => {
    onChange();
    revisionRef.current += 1;
    const revision = revisionRef.current;
    setStoredState((current) => {
      const reconciled = reconcileDraftState(current, {
        editorScopeKey,
        initialDraft,
        sourceScopeKey,
      });
      return {
        ...reconciled,
        draft: { ...reconciled.draft, [field]: value },
        revision,
      };
    });
  }, [editorScopeKey, initialDraft, onChange, sourceScopeKey]);

  const toggleTool = useCallback((toolName: string, kind: ToolListKind) => {
    onChange();
    revisionRef.current += 1;
    const revision = revisionRef.current;
    const field = TOOL_FIELD_BY_KIND[kind];
    setStoredState((current) => {
      const reconciled = reconcileDraftState(current, {
        editorScopeKey,
        initialDraft,
        sourceScopeKey,
      });
      const tools = reconciled.draft[field];
      const nextTools = tools.includes(toolName)
        ? tools.filter((name) => name !== toolName)
        : [...tools, toolName];
      return {
        ...reconciled,
        draft: { ...reconciled.draft, [field]: nextTools },
        revision,
      };
    });
  }, [editorScopeKey, initialDraft, onChange, sourceScopeKey]);

  const draftKey = buildAgentOptionsDraftKey(state.draft);
  return {
    draft: state.draft,
    draftKey,
    isDirty: draftKey !== buildAgentOptionsDraftKey(state.sourceDraft),
    revision: state.revision,
    revisionRef,
    toggleTool,
    updateField,
  };
}

function createDraftState(
  editorScopeKey: string,
  initialDraft: AgentOptionsDraft,
  sourceScopeKey: string,
): AgentOptionsDraftState {
  return {
    draft: initialDraft,
    editorScopeKey,
    revision: 0,
    sourceDraft: initialDraft,
    sourceScopeKey,
  };
}

function reconcileDraftState(
  current: AgentOptionsDraftState,
  next: AgentOptionsDraftSourceState,
): AgentOptionsDraftState {
  if (current.editorScopeKey !== next.editorScopeKey) {
    return createDraftState(
      next.editorScopeKey,
      next.initialDraft,
      next.sourceScopeKey,
    );
  }
  if (current.sourceScopeKey === next.sourceScopeKey) {
    return current;
  }
  return {
    draft: reconcileAgentOptionsDraft(
      current.draft,
      current.sourceDraft,
      next.initialDraft,
    ),
    editorScopeKey: next.editorScopeKey,
    revision: current.revision,
    sourceDraft: next.initialDraft,
    sourceScopeKey: next.sourceScopeKey,
  };
}
