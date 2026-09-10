/**
 * INPUT: 分组配对、Agent 目录与配对写命令。
 * OUTPUT: 名称旁的授权状态、外部对象摘要、共享 Agent 选择文字/缺项绑定与按需展开的技术详情。
 * POS: 配对目录列表纯视图；外部身份属于管理对象，内部绑定键才延后展示。
 */
"use client";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import {
  Check,
  Copy,
  Trash2,
  X,
  type LucideIcon,
} from "lucide-react";
import type { TranslationKey } from "@/shared/i18n/messages";
import { useId } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { buildAgentSelectionOptions, includeUnavailableAgentSelection, type AgentSelectionOption } from "@/lib/agent-selection-options";

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
  getPairingLabels,
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
  labelKey: TranslationKey;
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
  active: [{ labelKey: "capability.pairing_disable", status: "disabled" }],
  disabled: [{
    icon: Check,
    labelKey: "capability.pairing_approve",
    status: "active",
    tone: "primary",
    variant: "solid",
  }],
  pending: [
    {
      icon: Check,
      labelKey: "capability.pairing_approve",
      status: "active",
      tone: "primary",
      variant: "solid",
    },
    {
      icon: X,
      labelKey: "capability.pairing_reject",
      status: "rejected",
      tone: "danger",
      variant: "surface",
    },
  ],
  rejected: [{
    icon: Check,
    labelKey: "capability.pairing_approve",
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
  const { t } = useI18n();
  const agentOptions = buildAgentSelectionOptions(agents, t);
  return (
    <div className="space-y-5">
      {pendingItems.length > 0 ? (
        <PairingSection
          agentOptions={agentOptions}
          busy={busy}
          description={t("capability.pairing_pending_hint")}
          items={pendingItems}
          onCopySessionKey={onCopySessionKey}
          onDeletePairing={onDeletePairing}
          onUpdatePairing={onUpdatePairing}
          title={t("capability.pairing_status_pending")}
        />
      ) : null}
      {groups.map((group) => (
        <PairingSection
          agentOptions={agentOptions}
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
  agentOptions,
  busy,
  description,
  items,
  onCopySessionKey,
  onDeletePairing,
  onUpdatePairing,
  title,
}: {
  agentOptions: AgentSelectionOption[];
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
            agentOptions={agentOptions}
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
  agentOptions,
  busy,
  item,
  onCopySessionKey,
  onDeletePairing,
  onUpdatePairing,
}: {
  agentOptions: AgentSelectionOption[];
  busy: boolean;
  item: PairingView;
  onCopySessionKey: PairingListProps["onCopySessionKey"];
  onDeletePairing: PairingListProps["onDeletePairing"];
  onUpdatePairing: PairingListProps["onUpdatePairing"];
}) {
  const { t } = useI18n();
  const { locale } = useI18n();
  const labels = getPairingLabels(t);
  const agentFieldId = useId();
  const bindingKey = pairingBindingKey(item, labels);
  const sessionKey = pairingSessionKey(item);
  const activityAt = item.last_message_at || item.updated_at;
  return (
    <UiPanel className="overflow-hidden" padding="none" radius="sm">
      <div className="grid grid-cols-[minmax(0,1.2fr)_minmax(220px,0.7fr)_auto] items-center gap-3 px-3 py-3 max-lg:grid-cols-1">
        <div className="flex min-w-0 items-center gap-2.5">
          <ChannelIcon type={item.channel_type} />
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              <div className={cn(
                "min-w-0 max-w-full truncate",
                getUiTypographyClassName({
                  role: "control",
                  tone: "strong",
                  weight: "medium",
                }),
              )}>
                {pairingDisplayName(item, labels)}
              </div>
              <UiBadge tone={STATUS_TONES[item.status]}>
                {labels.statuses[item.status]}
              </UiBadge>
            </div>
            <div className={cn(
              "mt-1 flex min-w-0 items-center gap-1.5 overflow-hidden",
              getUiTypographyClassName({ role: "metadata", tone: "muted" }),
            )}>
              <span className="shrink-0">
                {labels.channels[item.channel_type] ?? item.channel_type}
              </span>
              <span aria-hidden="true">·</span>
              <span className="shrink-0">
                {labels.chatTypes[item.chat_type] ?? item.chat_type}
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
                {t(item.last_message_at ? "capability.pairing_last_message" : "capability.pairing_updated")}{" "}
                {formatPairingTime(activityAt, locale)}
              </span>
            </div>
          </div>
        </div>

        <UiField className="min-w-0" htmlFor={agentFieldId} label={t("capability.pairing_agent")}>
          <UiSelectMenu
            ariaLabel={t("capability.pairing_select_row_agent")}
            disabled={busy}
            id={agentFieldId}
            onChange={(value) => void onUpdatePairing(item, { agent_id: value })}
            options={includeUnavailableAgentSelection(agentOptions, item.agent_id, t)}
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
                {t(transition.labelKey)}
              </UiButton>
            );
          })}
          <UiIconButton
            disabled={busy}
            onClick={() => onDeletePairing(item)}
            size="lg"
            title={t("common.delete")}
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
        label={t("capability.pairing_details")}
        summaryRole="metadata"
        summaryTone="muted"
        variant="section"
      >
          <PairingTechnicalField label={t("capability.pairing_binding_key")} value={bindingKey} />
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
                disabled={busy || !sessionKey}
                onClick={() => void onCopySessionKey(item)}
                size="xs"
                title={t("capability.pairing_copy_session")}
                type="button"
                variant="ghost"
              >
                <Copy className="h-3.5 w-3.5" />
              </UiIconButton>
            </div>
            <UiTooltip label={sessionKey || t("capability.pairing_no_session")}><div
              className={cn(
                "truncate",
                getUiTypographyClassName({ role: "code", tone: "default" }),
              )}

            >
              {sessionKey || t("capability.pairing_no_session")}
            </div></UiTooltip>
          </div>
          <div className={cn(
            "min-w-0",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            <div>{t("capability.pairing_source")}: {item.source === "ingress" ? t("capability.pairing_first_message") : item.source}</div>
            <div>{t("capability.pairing_updated")}: {formatPairingTime(item.updated_at, locale)}</div>
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
      <UiTooltip label={value}><div
        className={cn(
          "truncate",
          getUiTypographyClassName({ role: "code", tone: "default" }),
        )}
      >
        {value}
      </div></UiTooltip>
    </div>
  );
}

function formatPairingTime(value: string, locale: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(locale, {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}
