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
  busy_id: string | null;
  groups: PairingGroup[];
  on_copy_session_key: (item: PairingView) => void | Promise<void>;
  on_delete_pairing: (item: PairingView) => void;
  on_update_pairing: (
    item: PairingView,
    next: { status?: ImPairingStatus; agent_id?: string },
  ) => void | Promise<void>;
}

function status_tone(status: ImPairingStatus): UiBadgeTone {
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

function format_target(item: PairingView) {
  const thread = item.thread_id ? ` / ${item.thread_id}` : "";
  return `${item.external_ref}${thread}`;
}

function chat_type_label(item: PairingView) {
  return CHAT_TYPE_OPTIONS.find((option) => option.value === item.chat_type)?.label ?? item.chat_type;
}

function binding_key(item: PairingView) {
  return [
    CHANNEL_LABELS[item.channel_type] ?? item.channel_type,
    item.account_id || "default",
    chat_type_label(item),
    item.external_ref,
    item.thread_id || "-",
  ].join(" / ");
}

function session_key_for_pairing(item: PairingView) {
  return item.session_key || "";
}

export function PairingList({
  agents,
  busy_id,
  groups,
  on_copy_session_key,
  on_delete_pairing,
  on_update_pairing,
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
                class_name="grid grid-cols-[minmax(0,1.2fr)_minmax(210px,0.8fr)_minmax(260px,1fr)_auto] items-center gap-4 max-2xl:grid-cols-[minmax(0,1fr)_minmax(240px,1fr)] max-lg:grid-cols-1"
                key={item.pairing_id}
              >
                <div className="min-w-0">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <UiBadge>{CHANNEL_LABELS[item.channel_type] ?? item.channel_type}</UiBadge>
                    <UiBadge tone={status_tone(item.status)}>
                      {STATUS_LABELS[item.status]}
                    </UiBadge>
                    <UiBadge>{chat_type_label(item)}</UiBadge>
                    {item.account_id ? <UiBadge tone="default">{item.account_id}</UiBadge> : null}
                  </div>
                  <div className="mt-2 truncate text-[16px] font-bold text-(--text-strong)">
                    {item.external_name || format_target(item)}
                  </div>
                  <div className="mt-1 truncate font-mono text-[12px] text-(--text-muted)">
                    {format_target(item)}
                  </div>
                  {item.account_id ? (
                    <div className="mt-1 truncate font-mono text-[11px] text-(--text-soft)" title={item.account_id}>
                      account: {item.account_id}
                    </div>
                  ) : null}
                </div>

                <UiField class_name="min-w-0" label="处理智能体">
                  <UiSelectMenu
                    aria_label="选择配对处理智能体"
                    disabled={busy_id === item.pairing_id}
                    on_change={(value) => void on_update_pairing(item, { agent_id: value })}
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
                    <div className="truncate font-mono text-(--text-default)" title={binding_key(item)}>
                      {binding_key(item)}
                    </div>
                  </div>
                  <div className="min-w-0">
                    <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase text-(--text-soft)">
                      <span>IM Session</span>
                      <UiIconButton
                        class_name="h-6 w-6"
                        disabled={busy_id === item.pairing_id}
                        onClick={() => void on_copy_session_key(item)}
                        size="sm"
                        title="复制 IM session key"
                        type="button"
                        variant="ghost"
                      >
                        <Copy className="h-3.5 w-3.5" />
                      </UiIconButton>
                    </div>
                    <div className="truncate font-mono text-(--text-default)" title={session_key_for_pairing(item)}>
                      {session_key_for_pairing(item)}
                    </div>
                  </div>
                  <div className="truncate">
                    来源：{item.source === "ingress" ? "首次消息" : item.source} · 更新：{new Date(item.updated_at).toLocaleString()}
                  </div>
                </div>

                <div className="flex items-center justify-end gap-2 max-lg:justify-start">
                  {item.status !== "active" ? (
                    <UiButton
                      disabled={busy_id === item.pairing_id}
                      onClick={() => void on_update_pairing(item, { status: "active" })}
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
                      disabled={busy_id === item.pairing_id}
                      onClick={() => void on_update_pairing(item, { status: "rejected" })}
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
                      disabled={busy_id === item.pairing_id}
                      onClick={() => void on_update_pairing(item, { status: "disabled" })}
                      size="sm"
                      type="button"
                    >
                      停用
                    </UiButton>
                  ) : null}
                  <UiIconButton
                    disabled={busy_id === item.pairing_id}
                    onClick={() => on_delete_pairing(item)}
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
