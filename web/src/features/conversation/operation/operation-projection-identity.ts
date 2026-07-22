/**
 * INPUT: Projected operation records and the conversation's authoritative Agent identity.
 * OUTPUT: Records with missing SDK Agent IDs filled without mutating source data.
 * POS: Projection identity boundary shared by narrative and runtime event streams.
 */

export function applyProjectedAgentIdentity<T extends { agent_id: string }>(
  records: readonly T[],
  fallbackAgentId?: string | null,
): T[] {
  const fallback_agent_id = fallbackAgentId?.trim() ?? "";
  if (!fallback_agent_id) {
    return records.slice();
  }
  return records.map((record) => (
    record.agent_id.trim()
      ? record
      : { ...record, agent_id: fallback_agent_id }
  ));
}
