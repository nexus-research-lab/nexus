"use client";

import { Check, Copy, Trash2, X } from "lucide-react";

import {
  ImPairingStatus,
  PairingView,
} from "@/lib/api/channel-api";
import { UiBadge } from "@/shared/ui/badge";
import type { UiBadgeTone } from "@/shared/ui/badge-styles";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { UiField } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import type { Agent } from "@/types/agent/agent";
import { CHANNEL_LABELS, CHAT_TYPE_OPTIONS, STATUS_LABELS } from "./pairing-options";

export interface PairingGroup {
  agent_id: string;
  agent_name: string;
  items: PairingView[];
}

interface PairingListProps {
  agents: Agent[];
  busyId: string | null;
  groups: PairingGroup[];
  onCopySessionKey: (item: PairingView) => void | Promise<void>;
  onDeletePairing: (item: PairingView) => void;
  onUpdatePairing: (
    item: PairingView,
    next: { status?: ImPairingStatus; agentId?: string },
  ) => void | Promise<void>;
}

function statusTone(status: ImPairingStatus): UiBadgeTone {
  switch (status) {
  case "active":
    return "success";
  case "pending":
    return "warning";
  case "rejected":
    return "danger";
  default:
    return "default";
  }
}

function formatTarget(item: PairingView) {
  const thread = item.thread_id ? ` / ${item.thread_id}` : "";
  return `${item.external_ref}${thread}`;
}

function chatTypeLabel(item: PairingView) {
  return CHAT_TYPE_OPTIONS.find((option) => option.value === item.chat_type)?.label ?? item.chat_type;
}

function bindingKey(item: PairingView) {
  return [
    CHANNEL_LABELS[item.channel_type] ?? item.channel_type,
    item.account_id || "default",
    chatTypeLabel(item),
    item.external_ref,
    item.thread_id || "-",
  ].join(" / ");
}

function sessionKeyForPairing(item: PairingView) {
  return item.session_key || "";
}

export function PairingList({
  agents,
  busyId: busyId,
  groups,
  onCopySessionKey: onCopySessionKey,
  onDeletePairing: onDeletePairing,
  onUpdatePairing: onUpdatePairing,
}: PairingListProps) {
  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <section className="space-y-2.5" key={group.agent_id}>
          <div className="flex items-center justify-between border-b border-(--divider-subtle-color) pb-2">
            <div className="min-w-0">
              <h2 className="truncate text-[15px] font-semibold text-(--text-strong)">
                {group.agent_name}
              </h2>
              <p className="truncate text-[12px] text-(--text-muted)">
                {group.items.length} 个外部对象绑定到此智能体
              </p>
            </div>
            <UiBadge tone="default">{group.agent_id}</UiBadge>
          </div>
          <div className="space-y-2.5">
            {group.items.map((item) => (
              <UiPanel
                className="grid grid-cols-[minmax(0,1.2fr)_minmax(210px,0.8fr)_minmax(260px,1fr)_auto] items-center gap-4 max-2xl:grid-cols-[minmax(0,1fr)_minmax(240px,1fr)] max-lg:grid-cols-1"
                key={item.pairing_id}
              >
                <div className="min-w-0">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <UiBadge>{CHANNEL_LABELS[item.channel_type] ?? item.channel_type}</UiBadge>
                    <UiBadge tone={statusTone(item.status)}>
                      {STATUS_LABELS[item.status]}
                    </UiBadge>
                    <UiBadge>{chatTypeLabel(item)}</UiBadge>
                    {item.account_id ? <UiBadge tone="default">{item.account_id}</UiBadge> : null}
                  </div>
                  <div className="mt-2 truncate text-[16px] font-bold text-(--text-strong)">
                    {item.external_name || formatTarget(item)}
                  </div>
                  <div className="mt-1 truncate font-mono text-[12px] text-(--text-muted)">
                    {formatTarget(item)}
                  </div>
                  {item.account_id ? (
                    <div className="mt-1 truncate font-mono text-[11px] text-(--text-soft)" title={item.account_id}>
                      account: {item.account_id}
                    </div>
                  ) : null}
                </div>

                <UiField className="min-w-0" label="处理智能体">
                  <UiSelectMenu
                    ariaLabel="选择配对处理智能体"
                    disabled={busyId === item.pairing_id}
                    onChange={(value) => void onUpdatePairing(item, { agentId: value })}
                    options={agents.map((agent) => ({
                      value: agent.agent_id,
                      label: agent.name,
                    }))}
                    size="sm"
                    value={item.agent_id}
                  />
                </UiField>

                <div className="min-w-0 space-y-1.5 text-[12px] leading-5 text-(--text-muted)">
                  <div className="min-w-0">
                    <div className="text-[11px] font-semibold uppercase text-(--text-soft)">绑定键</div>
                    <div className="truncate font-mono text-(--text-default)" title={bindingKey(item)}>
                      {bindingKey(item)}
                    </div>
                  </div>
                  <div className="min-w-0">
                    <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase text-(--text-soft)">
                      <span>IM Session</span>
                      <UiIconButton
                        className="h-6 w-6"
                        disabled={busyId === item.pairing_id}
                        onClick={() => void onCopySessionKey(item)}
                        size="sm"
                        title="复制 IM session key"
                        type="button"
                        variant="ghost"
                      >
                        <Copy className="h-3.5 w-3.5" />
                      </UiIconButton>
                    </div>
                    <div className="truncate font-mono text-(--text-default)" title={sessionKeyForPairing(item)}>
                      {sessionKeyForPairing(item)}
                    </div>
                  </div>
                  <div className="truncate">
                    来源：{item.source === "ingress" ? "首次消息" : item.source} · 更新：{new Date(item.updated_at).toLocaleString()}
                  </div>
                </div>

                <div className="flex items-center justify-end gap-2 max-lg:justify-start">
                  {item.status !== "active" ? (
                    <UiButton
                      disabled={busyId === item.pairing_id}
                      onClick={() => void onUpdatePairing(item, { status: "active" })}
                      size="sm"
                      tone="primary"
                      type="button"
                      variant="solid"
                    >
                      <Check className="h-3.5 w-3.5" />
                      通过
                    </UiButton>
                  ) : null}
                  {item.status === "pending" ? (
                    <UiButton
                      disabled={busyId === item.pairing_id}
                      onClick={() => void onUpdatePairing(item, { status: "rejected" })}
                      size="sm"
                      tone="danger"
                      type="button"
                      variant="surface"
                    >
                      <X className="h-3.5 w-3.5" />
                      拒绝
                    </UiButton>
                  ) : null}
                  {item.status === "active" ? (
                    <UiButton
                      disabled={busyId === item.pairing_id}
                      onClick={() => void onUpdatePairing(item, { status: "disabled" })}
                      size="sm"
                      type="button"
                    >
                      停用
                    </UiButton>
                  ) : null}
                  <UiIconButton
                    disabled={busyId === item.pairing_id}
                    onClick={() => onDeletePairing(item)}
                    size="lg"
                    title="删除"
                    tone="danger"
                    type="button"
                    variant="ghost"
                  >
                    <Trash2 className="h-4 w-4" />
                  </UiIconButton>
                </div>
              </UiPanel>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
