import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
  SpotlightToken,
} from "@/types/app/launcher";

import type {
  LauncherMentionTarget,
  RecentLauncherEntry,
} from "./launcher-console-types";

const TOKEN_SWATCHES = [
  { fill: "#5FA052", text: "#FFFFFF", ring: "#8DBA86" },
  { fill: "#E8A838", text: "#FFFFFF", ring: "#F0C56C" },
  { fill: "#4DAA9F", text: "#FFFFFF", ring: "#7CC8BE" },
  { fill: "#A78BFA", text: "#FFFFFF", ring: "#C2B0FF" },
  { fill: "#6C7BDB", text: "#FFFFFF", ring: "#9AA4F2" },
  { fill: "#D4687A", text: "#FFFFFF", ring: "#E597A3" },
  { fill: "#C4A86B", text: "#FFFFFF", ring: "#D7C08D" },
  { fill: "#8B9089", text: "#FFFFFF", ring: "#B6BAB4" },
  { fill: "#E8945A", text: "#FFFFFF", ring: "#F0B186" },
];

function getInitials(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return "AG";
  }
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return `${parts[0][0] ?? ""}${parts[1][0] ?? ""}`.toUpperCase();
}

export function truncateLauncherChipLabel(
  label: string,
  maxChars: number = 10,
): string {
  const chars = Array.from(label.trim());
  if (chars.length <= maxChars) {
    return label.trim();
  }

  // Hero 推荐项空间很窄，超长名称改为中间省略。
  const headCount = Math.max(2, Math.ceil((maxChars - 1) / 2));
  const tailCount = Math.max(2, maxChars - 1 - headCount);
  return `${chars.slice(0, headCount).join("")}…${chars.slice(-tailCount).join("")}`;
}

export function isLauncherChipTruncated(
  label: string,
  maxChars: number = 6,
): boolean {
  return Array.from(label.trim()).length > maxChars;
}

export function buildDecorativeTokens(
  agents: LauncherAgentSummary[],
  rooms: LauncherRoomSummary[],
): SpotlightToken[] {
  const agentTokens: SpotlightToken[] = agents.map((agent, index) => ({
    key: `agent-${agent.id}`,
    label: getInitials(agent.name),
    agent_id: agent.id,
    kind: "agent" as const,
    swatch: TOKEN_SWATCHES[index % TOKEN_SWATCHES.length],
  }));

  const roomTokens: SpotlightToken[] = rooms
    .filter((room) => room.room_type === "room")
    .sort(
      (left, right) =>
        getLauncherRoomTimestamp(right) - getLauncherRoomTimestamp(left),
    )
    .slice(0, 8)
    .map((room, index) => ({
      key: `room-${room.id}`,
      label: getInitials(room.name?.trim() || "Room"),
      agent_id: null,
      kind: "room" as const,
      swatch:
        TOKEN_SWATCHES[(agentTokens.length + index) % TOKEN_SWATCHES.length],
    }));

  const fallback = [
    { label: "SA", kind: "agent" as const },
    { label: "NV", kind: "agent" as const },
    { label: "BO", kind: "agent" as const },
    { label: "DX", kind: "room" as const },
    { label: "WR", kind: "room" as const },
    { label: "QA", kind: "room" as const },
    { label: "SP", kind: "room" as const },
    { label: "AR", kind: "room" as const },
    { label: "NO", kind: "agent" as const },
    { label: "PR", kind: "agent" as const },
    { label: "FL", kind: "agent" as const },
    { label: "PI", kind: "agent" as const },
    { label: "RL", kind: "room" as const },
    { label: "AT", kind: "agent" as const },
  ];

  const source: SpotlightToken[] = [...agentTokens, ...roomTokens];
  fallback.forEach((item, index) => {
    if (source.length < 18) {
      source.push({
        key: `fallback-${item.label}-${index}`,
        label: item.label,
        agent_id: null,
        kind: item.kind,
        swatch:
          TOKEN_SWATCHES[
            (agentTokens.length + roomTokens.length + index) %
              TOKEN_SWATCHES.length
          ],
      });
    }
  });

  return source.slice(0, 12);
}

export function buildLauncherMentionTargets(
  agents: LauncherAgentSummary[],
  rooms: LauncherRoomSummary[],
): LauncherMentionTarget[] {
  const agentTargets = agents
    .slice()
    .sort((left, right) => left.name.localeCompare(right.name, "zh-CN"))
    .map((agent) => ({
      id: `agent-${agent.id}`,
      label: agent.name,
      marker: agent.name.charAt(0).toUpperCase(),
      subtitle: "Agent",
      kind: "agent" as const,
    }));

  const roomTargets = rooms
    .filter((room) => room.room_type === "room")
    .sort(
      (left, right) =>
        getLauncherRoomTimestamp(right) - getLauncherRoomTimestamp(left),
    )
    .map((room) => ({
      id: `room-${room.id}`,
      label: room.name?.trim() || "未命名 Room",
      marker: "#",
      subtitle: "Room",
      kind: "room" as const,
    }));

  return [...agentTargets, ...roomTargets];
}


export function buildRecentLauncherEntries(
  conversations: LauncherConversationSummary[],
): RecentLauncherEntry[] {
  return conversations
    .slice()
    .sort(
      (left, right) =>
        getLauncherConversationTimestamp(right) -
        getLauncherConversationTimestamp(left),
    )
    .map((conversation) => ({
      key: conversation.session_key,
      type: conversation.room_type,
      label:
        conversation.title.trim() ||
        (conversation.room_type === "dm" ? "未命名会话" : "未命名话题"),
      last_activity_at: getLauncherConversationTimestamp(conversation),
      agent_id: conversation.agent_id,
      room_id: conversation.room_id,
      conversation_id: conversation.conversation_id,
    }))
    .filter(
      (entry) => Boolean(entry.conversation_id) || Boolean(entry.agent_id),
    )
    .slice(0, 3);
}

function getLauncherRoomTimestamp(room: LauncherRoomSummary): number {
  return new Date(room.updated_at ?? room.created_at ?? 0).getTime();
}

function getLauncherConversationTimestamp(
  conversation: LauncherConversationSummary,
): number {
  return new Date(conversation.last_activity ?? 0).getTime();
}
