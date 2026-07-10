import type {
  CreatePairingPayload,
  ImChannelType,
  ImChatType,
  ImPairingStatus,
  PairingView,
} from "@/lib/api/channel-api";
import type { Agent } from "@/types/agent/agent";

import {
  CHANNEL_LABELS,
  CHAT_TYPE_LABELS,
} from "./pairing-options";

export interface PairingGroup {
  agent_id: string;
  agent_name: string;
  items: PairingView[];
}

export interface PairingFilters {
  agentId: string;
  channel: ImChannelType | "";
  query: string;
  status: ImPairingStatus | "";
}

export interface CreatePairingDraft {
  accountId: string;
  agentId: string;
  channelType: ImChannelType;
  chatType: ImChatType;
  externalName: string;
  externalRef: string;
  status: ImPairingStatus;
  threadId: string;
}

export function createPairingDraft(agentId: string): CreatePairingDraft {
  return {
    accountId: "",
    agentId,
    channelType: "feishu",
    chatType: "dm",
    externalName: "",
    externalRef: "",
    status: "active",
    threadId: "",
  };
}

export function buildCreatePairingPayload(
  draft: CreatePairingDraft,
): CreatePairingPayload | null {
  const externalRef = draft.externalRef.trim();
  if (!externalRef || !draft.agentId) {
    return null;
  }
  return {
    account_id: draft.accountId.trim() || undefined,
    agent_id: draft.agentId,
    channel_type: draft.channelType,
    chat_type: draft.chatType,
    external_name: draft.externalName.trim() || undefined,
    external_ref: externalRef,
    status: draft.status,
    thread_id: draft.threadId.trim() || undefined,
  };
}

export function filterPairings(
  items: PairingView[],
  query: string,
): PairingView[] {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return items;
  }
  return items.filter((item) => pairingSearchValues(item).some(
    (value) => value.toLowerCase().includes(normalizedQuery),
  ));
}

export function groupPairings(
  items: PairingView[],
  agents: Agent[],
): PairingGroup[] {
  const agentNames = new Map(agents.map((agent) => [agent.agent_id, agent.name]));
  const groups = new Map<string, PairingGroup>();
  items.forEach((item) => {
    const group = groups.get(item.agent_id);
    if (group) {
      group.items.push(item);
      return;
    }
    groups.set(item.agent_id, {
      agent_id: item.agent_id,
      agent_name: item.agent_name || agentNames.get(item.agent_id) || item.agent_id,
      items: [item],
    });
  });
  return Array.from(groups.values()).sort(
    (left, right) => left.agent_name.localeCompare(right.agent_name),
  );
}

export function countPairingStatus(
  items: PairingView[],
  status: ImPairingStatus,
): number {
  return items.filter((item) => item.status === status).length;
}

export function pairingDisplayName(item: PairingView): string {
  return item.external_name || item.external_ref;
}

export function pairingTarget(item: PairingView): string {
  const thread = item.thread_id ? ` / ${item.thread_id}` : "";
  return `${item.external_ref}${thread}`;
}

export function pairingBindingKey(item: PairingView): string {
  return [
    CHANNEL_LABELS[item.channel_type] ?? item.channel_type,
    item.account_id || "default",
    CHAT_TYPE_LABELS[item.chat_type] ?? item.chat_type,
    item.external_ref,
    item.thread_id || "-",
  ].join(" / ");
}

export function pairingSessionKey(item: PairingView): string {
  return item.session_key || "";
}

function pairingSearchValues(item: PairingView): string[] {
  return [
    item.external_name ?? "",
    item.external_ref,
    item.account_id ?? "",
    item.thread_id ?? "",
    pairingSessionKey(item),
    item.agent_name ?? "",
    CHANNEL_LABELS[item.channel_type] ?? item.channel_type,
  ];
}
