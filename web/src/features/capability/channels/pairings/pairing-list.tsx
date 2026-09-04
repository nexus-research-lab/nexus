/**
 * INPUT: 分组配对、Agent 目录与配对写命令。
 * OUTPUT: 可识别的外部对象摘要和按需展开的内部技术详情。
 * POS: 配对目录列表纯视图；外部身份属于管理对象，内部绑定键才延后展示。
 */
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
import { CapabilitySectionHeader } from "@/features/capability/shared/capability-page-layout";
import { UiBadge } from "@/shared/ui/display/badge";
import type { UiBadgeTone } from "@/shared/ui/display/badge-styles";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiField } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { Agent } from "@/types/agent/agent";

import { ChannelIcon } from "../channel-icon";
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
  pendingItems: PairingView[];
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
  pendingItems,
  onCopySessionKey,
  onDeletePairing,
  onUpdatePairing,
}: PairingListProps) {
  return (
    <div className="space-y-5">
      {pendingItems.length > 0 ? (
        <PairingSection
          agents={agents}
          busy={busy}
          description="首次消息正在等待授权"
          items={pendingItems}
          onCopySessionKey={onCopySessionKey}
          onDeletePairing={onDeletePairing}
          onUpdatePairing={onUpdatePairing}
          title="待处理"
        />
      ) : null}
      {groups.map((group) => (
        <PairingSection
          agents={agents}
          busy={busy}
          items={group.items}
          key={group.agent_id}
          onCopySessionKey={onCopySessionKey}
          onDeletePairing={onDeletePairing}
          onUpdatePairing={onUpdatePairing}
          title={group.agent_name}
        />
      ))}
    </div>
  );
}

function PairingSection({
  agents,
  busy,
  description,
  items,
  onCopySessionKey,
  onDeletePairing,
  onUpdatePairing,
  title,
}: {
  agents: Agent[];
  busy: boolean;
  description?: string;
  items: PairingView[];
  onCopySessionKey: PairingListProps["onCopySessionKey"];
  onDeletePairing: PairingListProps["onDeletePairing"];
  onUpdatePairing: PairingListProps["onUpdatePairing"];
  title: string;
}) {
  return (
    <section className="space-y-2">
      <CapabilitySectionHeader
        count={String(items.length)}
        description={description}
        title={title}
      />
      <div className="space-y-2">
        {items.map((item) => (
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
  const activityAt = item.last_message_at || item.updated_at;
  return (
    <UiPanel className="overflow-hidden" padding="none" radius="sm">
      <div className="grid grid-cols-[minmax(0,1.2fr)_minmax(220px,0.7fr)_auto] items-center gap-3 px-3 py-3 max-lg:grid-cols-1">
        <div className="flex min-w-0 items-center gap-2.5">
          <ChannelIcon type={item.channel_type} />
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-2">
              <div className={cn(
                "min-w-0 flex-1 truncate",
                getUiTypographyClassName({
                  role: "control",
                  tone: "strong",
                  weight: "medium",
                }),
              )}>
                {pairingDisplayName(item)}
              </div>
              <UiBadge tone={STATUS_TONES[item.status]}>
                {STATUS_LABELS[item.status]}
              </UiBadge>
            </div>
            <div className={cn(
              "mt-1 flex min-w-0 items-center gap-1.5 overflow-hidden",
              getUiTypographyClassName({ role: "metadata", tone: "muted" }),
            )}>
              <span className="shrink-0">
                {CHANNEL_LABELS[item.channel_type] ?? item.channel_type}
              </span>
              <span aria-hidden="true">·</span>
              <span className="shrink-0">
                {CHAT_TYPE_LABELS[item.chat_type] ?? item.chat_type}
              </span>
              <span aria-hidden="true">·</span>
              <span className={cn(
                "min-w-0 truncate",
                getUiTypographyClassName({ role: "code", tone: "muted" }),
              )}>
                {pairingTarget(item)}
              </span>
              <span aria-hidden="true">·</span>
              <span className="shrink-0 text-(--text-soft)">
                {item.last_message_at ? "最近消息" : "更新于"}{" "}
                {formatPairingTime(activityAt)}
              </span>
            </div>
          </div>
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
      </div>

      <UiDisclosure
        contentClassName="grid gap-3 md:grid-cols-[minmax(0,1.25fr)_minmax(0,1fr)_minmax(180px,0.6fr)]"
        density="compact"
        inset="sm"
        label="技术详情"
        summaryRole="metadata"
        summaryTone="muted"
        variant="section"
      >
          <PairingTechnicalField label="绑定键" value={bindingKey} />
          <div className="min-w-0">
            <div className={cn(
              "flex h-6 items-center gap-1.5",
              getUiTypographyClassName({
                role: "caption",
                tone: "soft",
                weight: "semibold",
              }),
            )}>
              <span>IM Session</span>
              <UiIconButton
                className="h-6 w-6"
                disabled={busy || !sessionKey}
                onClick={() => void onCopySessionKey(item)}
                size="sm"
                title="复制 IM session key"
                type="button"
                variant="ghost"
              >
                <Copy className="h-3.5 w-3.5" />
              </UiIconButton>
            </div>
            <div
              className={cn(
                "truncate",
                getUiTypographyClassName({ role: "code", tone: "default" }),
              )}
              title={sessionKey || "未生成"}
            >
              {sessionKey || "未生成"}
            </div>
          </div>
          <div className={cn(
            "min-w-0",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            <div>来源：{item.source === "ingress" ? "首次消息" : item.source}</div>
            <div>更新：{formatPairingTime(item.updated_at)}</div>
          </div>
      </UiDisclosure>
    </UiPanel>
  );
}

function PairingTechnicalField({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className={cn(
        "flex h-6 items-center",
        getUiTypographyClassName({
          role: "caption",
          tone: "soft",
          weight: "semibold",
        }),
      )}>
        {label}
      </div>
      <div
        className={cn(
          "truncate",
          getUiTypographyClassName({ role: "code", tone: "default" }),
        )}
        title={value}
      >
        {value}
      </div>
    </div>
  );
}

function formatPairingTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}
