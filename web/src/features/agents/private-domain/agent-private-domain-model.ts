import { AgentPrivateEvent, AgentPrivateThread } from "@/types/agent/private-domain";

export function privateThreadTitle(thread: AgentPrivateThread, agentId: string) {
  const peers = thread.participants.filter((participant) => participant.agent_id !== agentId);
  if (peers.length === 0) {
    return "私有笔记";
  }
  return peers.map((participant) => participant.name || participant.agent_id).join("、");
}

export function eventRouteLabel(event: AgentPrivateEvent, agentId: string) {
  const recipients = event.recipients
    .map((recipientId) => {
      const participant = event.participants.find((item) => item.agent_id === recipientId);
      return recipientId === agentId ? "我" : participant?.name || recipientId;
    })
    .filter(Boolean);
  if (recipients.length > 0) {
    return `给 ${recipients.join("、")}`;
  }
  if (event.reply_route?.mode === "private") {
    const replyRecipients = (event.reply_route.recipients ?? []).map((recipientId) => {
      const participant = event.participants.find((item) => item.agent_id === recipientId);
      return recipientId === agentId ? "我" : participant?.name || recipientId;
    });
    return replyRecipients.length ? `回复到 ${replyRecipients.join("、")}` : "私密回复";
  }
  if (event.reply_route?.mode === "public") {
    return "回复到公区";
  }
  if (event.reply_route?.mode === "none") {
    return "不要求回复";
  }
  return scopeLabel(event.direction);
}

function scopeLabel(scope: string) {
  switch (scope) {
    case "direct":
      return "一对一";
    case "audience":
      return "小范围";
    case "self":
      return "仅自己";
    default:
      return scope || "-";
  }
}
