import type { AgentPrivateThread } from "@/types/agent/private-domain";

export function privateThreadTitle(
  thread: AgentPrivateThread,
  agentId: string,
): string {
  const peers = thread.participants.filter(
    (participant) => participant.agent_id !== agentId,
  );
  if (peers.length === 0) {
    return "私有笔记";
  }
  return peers
    .map((participant) => participant.name || participant.agent_id)
    .join("、");
}
