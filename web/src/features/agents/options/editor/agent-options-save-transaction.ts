export interface AgentOptionsSaveToken {
  commandScopeKey: string;
  draftRevision: number;
  id: number;
  sourceScopeKey: string;
}

interface AgentOptionsSaveState {
  commandScopeKey: string;
  draftRevision: number;
  sourceScopeKey: string;
  token: AgentOptionsSaveToken | null;
}

export function isAgentOptionsSaveCurrent(
  expected: AgentOptionsSaveToken,
  current: AgentOptionsSaveState,
  requireSourceScope: boolean,
): boolean {
  return current.token?.id === expected.id
    && current.commandScopeKey === expected.commandScopeKey
    && current.draftRevision === expected.draftRevision
    && (!requireSourceScope || current.sourceScopeKey === expected.sourceScopeKey);
}
