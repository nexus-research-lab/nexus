"use client";

import {
  Check,
  Copy,
  Trash2,
  X,
  type LucideIcon,
} from "lucide-react";

import type {
  ImPairingStatus,
  PairingView,
  UpdatePairingPayload,
} from "@/lib/api/capability/channel-api";
import { UiBadge } from "@/shared/ui/display/badge";
import type { UiBadgeTone } from "@/shared/ui/display/badge-styles";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiField } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { Agent } from "@/types/agent/agent";

import {
  pairingBindingKey,
  pairingDisplayName,
  pairingSessionKey,
  pairingTarget,
  type PairingGroup,
} from "./pairing-model";
import {
  CHANNEL_LABELS,
  CHAT_TYPE_LABELS,
  STATUS_LABELS,
} from "./pairing-options";

interface PairingListProps {
  agents: Agent[];
  busy: boolean;
  groups: PairingGroup[];
  onCopySessionKey: (item: PairingView) => void | Promise<void>;
  onDeletePairing: (item: PairingView) => void;
  onUpdatePairing: (
    item: PairingView,
    next: UpdatePairingPayload,
  ) => void | Promise<void>;
}

interface PairingTransition {
  icon?: LucideIcon;
  label: string;
  status: ImPairingStatus;
  tone?: "danger" | "primary";
  variant?: "solid" | "surface";
}

const STATUS_TONES: Record<ImPairingStatus, UiBadgeTone> = {
  active: "success",
  disabled: "default",
  pending: "warning",
  rejected: "danger",
};

const PAIRING_TRANSITIONS: Record<ImPairingStatus, PairingTransition[]> = {
  active: [{ label: "停用", status: "disabled" }],
  disabled: [{
    icon: Check,
    label: "通过",
    status: "active",
    tone: "primary",
    variant: "solid",
  }],
  pending: [
    {
      icon: Check,
      label: "通过",
      status: "active",
      tone: "primary",
      variant: "solid",
    },
    {
      icon: X,
      label: "拒绝",
      status: "rejected",
      tone: "danger",
      variant: "surface",
    },
  ],
  rejected: [{
    icon: Check,
    label: "通过",
    status: "active",
    tone: "primary",
    variant: "solid",
  }],
};

export function PairingList({
  agents,
  busy,
  groups,
  onCopySessionKey,
  onDeletePairing,
  onUpdatePairing,
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
              <PairingRow
                agents={agents}
                busy={busy}
                item={item}
                key={item.pairing_id}
                onCopySessionKey={onCopySessionKey}
                onDeletePairing={onDeletePairing}
                onUpdatePairing={onUpdatePairing}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function PairingRow({
  agents,
  busy,
  item,
  onCopySessionKey,
  onDeletePairing,
  onUpdatePairing,
}: {
  agents: Agent[];
  busy: boolean;
  item: PairingView;
  onCopySessionKey: PairingListProps["onCopySessionKey"];
  onDeletePairing: PairingListProps["onDeletePairing"];
  onUpdatePairing: PairingListProps["onUpdatePairing"];
}) {
  const bindingKey = pairingBindingKey(item);
  const sessionKey = pairingSessionKey(item);
  return (
    <UiPanel className="grid grid-cols-[minmax(0,1.2fr)_minmax(210px,0.8fr)_minmax(260px,1fr)_auto] items-center gap-4 max-2xl:grid-cols-[minmax(0,1fr)_minmax(240px,1fr)] max-lg:grid-cols-1">
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <UiBadge>{CHANNEL_LABELS[item.channel_type] ?? item.channel_type}</UiBadge>
          <UiBadge tone={STATUS_TONES[item.status]}>
            {STATUS_LABELS[item.status]}
          </UiBadge>
          <UiBadge>{CHAT_TYPE_LABELS[item.chat_type] ?? item.chat_type}</UiBadge>
          {item.account_id ? <UiBadge tone="default">{item.account_id}</UiBadge> : null}
        </div>
        <div className="mt-2 truncate text-[16px] font-bold text-(--text-strong)">
          {pairingDisplayName(item)}
        </div>
        <div className="mt-1 truncate font-mono text-[12px] text-(--text-muted)">
          {pairingTarget(item)}
        </div>
        {item.account_id ? (
          <div
            className="mt-1 truncate font-mono text-[11px] text-(--text-soft)"
            title={item.account_id}
          >
            account: {item.account_id}
          </div>
        ) : null}
      </div>

      <UiField className="min-w-0" label="处理智能体">
        <UiSelectMenu
          ariaLabel="选择配对处理智能体"
          disabled={busy}
          onChange={(value) => void onUpdatePairing(item, { agent_id: value })}
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
          <div className="truncate font-mono text-(--text-default)" title={bindingKey}>
            {bindingKey}
          </div>
        </div>
        <div className="min-w-0">
          <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase text-(--text-soft)">
            <span>IM Session</span>
            <UiIconButton
              className="h-6 w-6"
              disabled={busy}
              onClick={() => void onCopySessionKey(item)}
              size="sm"
              title="复制 IM session key"
              type="button"
              variant="ghost"
            >
              <Copy className="h-3.5 w-3.5" />
            </UiIconButton>
          </div>
          <div className="truncate font-mono text-(--text-default)" title={sessionKey}>
            {sessionKey}
          </div>
        </div>
        <div className="truncate">
          来源：{item.source === "ingress" ? "首次消息" : item.source} · 更新：{new Date(item.updated_at).toLocaleString()}
        </div>
      </div>

      <div className="flex items-center justify-end gap-2 max-lg:justify-start">
        {PAIRING_TRANSITIONS[item.status].map((transition) => {
          const Icon = transition.icon;
          return (
            <UiButton
              disabled={busy}
              key={transition.status}
              onClick={() => void onUpdatePairing(item, {
                status: transition.status,
              })}
              size="sm"
              tone={transition.tone}
              type="button"
              variant={transition.variant}
            >
              {Icon ? <Icon className="h-3.5 w-3.5" /> : null}
              {transition.label}
            </UiButton>
          );
        })}
        <UiIconButton
          disabled={busy}
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
  );
}
