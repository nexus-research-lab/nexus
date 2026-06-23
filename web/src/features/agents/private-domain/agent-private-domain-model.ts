import { AgentPrivateEvent, AgentPrivateThread } from "@/types/agent/private-domain";

export function private_thread_title(thread: AgentPrivateThread, agent_id: string) {
  const peers = thread.participants.filter((participant) => participant.agent_id !== agent_id);
  if (peers.length === 0) {
    return "私有笔记";
  }
  return peers.map((participant) => participant.name || participant.agent_id).join("、");
}

export function event_route_label(event: AgentPrivateEvent, agent_id: string) {
  const recipients = event.recipients
    .map((recipient_id) => {
      const participant = event.participants.find((item) => item.agent_id === recipient_id);
      return recipient_id === agent_id ? "我" : participant?.name || recipient_id;
    })
    .filter(Boolean);
  if (recipients.length > 0) {
    return `给 ${recipients.join("、")}`;
  }
  if (event.reply_route?.mode === "private") {
    const reply_recipients = (event.reply_route.recipients ?? []).map((recipient_id) => {
      const participant = event.participants.find((item) => item.agent_id === recipient_id);
      return recipient_id === agent_id ? "我" : participant?.name || recipient_id;
    });
    return reply_recipients.length ? `回复到 ${reply_recipients.join("、")}` : "私密回复";
  }
  if (event.reply_route?.mode === "public") {
    return "回复到公区";
  }
  if (event.reply_route?.mode === "none") {
    return "不要求回复";
  }
  return scope_label(event.direction);
}

function scope_label(scope: string) {
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
