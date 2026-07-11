import { useCallback } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import type { AgentOptionsDraft } from "./agent-options-draft";

type AgentOptionsDraftField = keyof AgentOptionsDraft;
type ToolListKind = "allowed" | "disallowed";

const TOOL_FIELD_BY_KIND: Readonly<Record<ToolListKind, "allowedTools" | "disallowedTools">> = {
  allowed: "allowedTools",
  disallowed: "disallowedTools",
};

interface UseAgentOptionsDraftOptions {
  initialDraft: AgentOptionsDraft;
  onChange: () => void;
  scopeKey: string;
}

export function useAgentOptionsDraft({
  initialDraft,
  onChange,
  scopeKey,
}: UseAgentOptionsDraftOptions) {
  const [draft, setDraft] = useResettableState(initialDraft, scopeKey);

  const updateField = useCallback(<Field extends AgentOptionsDraftField>(
    field: Field,
    value: AgentOptionsDraft[Field],
  ) => {
    onChange();
    setDraft((current) => ({ ...current, [field]: value }));
  }, [onChange, setDraft]);

  const toggleTool = useCallback((toolName: string, kind: ToolListKind) => {
    onChange();
    const field = TOOL_FIELD_BY_KIND[kind];
    setDraft((current) => {
      const tools = current[field];
      const nextTools = tools.includes(toolName)
        ? tools.filter((name) => name !== toolName)
        : [...tools, toolName];
      return { ...current, [field]: nextTools };
    });
  }, [onChange, setDraft]);

  return { draft, toggleTool, updateField };
}
