/**
 * INPUT: Agent 身份与打开详情、私聊的页面命令。
 * OUTPUT: 单一目录卡片与列表行；完整身份和元信息共享投影，整卡打开详情、唯一次动作为聊天（发起群聊留在详情页）；权限与 Provider 只在偏离默认时显示。
 * POS: 联系人管理目录卡片；正文复用共享槽位的外部布局，默认层承担 Agent 选择所需的比较信息。
 */
"use client";

import { MessageCircle } from "lucide-react";

import { getInitialAgentOptions } from "@/config/runtime-options";
import { AGENT_PERMISSION_MODES } from "@/lib/agent-options";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { Agent } from "@/types/agent/agent";
import { formatProviderLabel } from "@/types/capability/provider";
import { WorkspaceCatalogTextAction } from "@/shared/ui/workspace/catalog/workspace-catalog-actions";
import { WorkspaceCatalogCard } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import {
  WorkspaceCatalogBody,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/catalog/workspace-catalog-content";

interface ContactsAgentCardProps {
  agent: Agent;
  onOpenProfile: () => void;
  onOpenRoom: () => void;
  view: "grid" | "list";
}

interface ContactsAgentCardViewProps extends Omit<ContactsAgentCardProps, "view"> {
  businessTags: string[];
  chatLabel: string;
  editLabel: string;
  /** 偏离新建 Agent 默认权限时的权限名；默认值不占卡片行。 */
  permissionMode: string | null;
  /** 显式指定的 Provider；跟随默认模型时为空。 */
  provider: string | null;
}

export function ContactsAgentCard({
  agent,
  onOpenProfile,
  onOpenRoom,
  view,
}: ContactsAgentCardProps) {
  const { t } = useI18n();

  const permissionValue = agent.options.permission_mode?.trim() || "";
  const defaultPermissionValue = getInitialAgentOptions().permission_mode ?? "";
  const permissionMode = permissionValue && permissionValue !== defaultPermissionValue
    ? AGENT_PERMISSION_MODES.find((option) => option.value === permissionValue) ?? null
    : null;
  const provider = agent.options.provider?.trim()
    ? formatProviderLabel(agent.options.provider)
    : null;
  const businessTags = [...new Set(
    (agent.business_tags ?? []).map((tag) => tag.trim()).filter(Boolean),
  )];

  const viewProps: ContactsAgentCardViewProps = {
    agent,
    businessTags,
    chatLabel: t("contacts.chat"),
    editLabel: t("common.edit"),
    onOpenProfile,
    onOpenRoom,
    permissionMode: permissionMode ? t(permissionMode.labelKey) : null,
    provider,
  };

  if (view === "list") {
    return <ContactsAgentListRow {...viewProps} />;
  }

  return <ContactsAgentGridCard {...viewProps} />;
}

function ContactsAgentListRow(props: ContactsAgentCardViewProps) {
  const { agent, businessTags, editLabel, onOpenProfile, permissionMode } = props;
  const { t } = useI18n();
  return (
    <UiListRow
      aria-label={`${editLabel} ${agent.name}`}
      className="items-start"
      variant="flush"
      leading={<UiAgentAvatar avatar={agent.avatar} name={agent.name} size="md" />}
      onClick={onOpenProfile}
      right={<ContactsAgentActions {...props} compact />}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
          <WorkspaceCatalogTitle className="min-w-0 [overflow-wrap:anywhere]" size="sm">
            {agent.name}
          </WorkspaceCatalogTitle>
          {permissionMode ? (
            <UiBadge className="max-w-full whitespace-normal [overflow-wrap:anywhere]" size="sm" tone="idle">
              {permissionMode}
            </UiBadge>
          ) : null}
          <ContactsAgentBusinessTags className="hidden md:flex" tags={businessTags} />
        </div>
        <ContactsAgentMetadata provider={props.provider} />
        <p className={cn("mt-1 truncate", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
          {agent.description || t("contacts.no_description")}
        </p>
      </div>
    </UiListRow>
  );
}

function ContactsAgentGridCard(props: ContactsAgentCardViewProps) {
  const { agent, businessTags, editLabel, onOpenProfile } = props;
  return (
    <WorkspaceCatalogCard
      align="center"
      className="h-full min-w-0"
      primaryAction={{ label: `${editLabel} ${agent.name}`, onClick: onOpenProfile }}
      size="comfort"
    >
      <div className="flex w-full min-w-0 flex-1 flex-col items-center">
        <UiAgentAvatar avatar={agent.avatar} name={agent.name} size="lg" />
        <WorkspaceCatalogBody className="mt-3 w-full">
          <WorkspaceCatalogTitle className="[overflow-wrap:anywhere]" size="lg">
            {agent.name}
          </WorkspaceCatalogTitle>
          {agent.description ? (
            <WorkspaceCatalogDescription className="mt-1.5" lines={2}>
              {agent.description}
            </WorkspaceCatalogDescription>
          ) : null}
          <ContactsAgentBusinessTags className="mt-2 justify-center" tags={businessTags} />
          <ContactsAgentMetadata {...props} className="justify-center" />
        </WorkspaceCatalogBody>
      </div>
      <WorkspaceCatalogFooter className="mt-2 w-full" justify="center">
        <ContactsAgentActions {...props} />
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}

function ContactsAgentMetadata({ provider, permissionMode, className }:
  Pick<ContactsAgentCardViewProps, "provider"> & { permissionMode?: string | null; className?: string }) {
  const { t } = useI18n();
  if (!provider && !permissionMode) {
    return null;
  }
  return <dl className={cn("mt-2 flex min-w-0 flex-wrap gap-x-3 gap-y-1",
    getUiTypographyClassName({ role: "metadata", tone: "muted" }), className)}>
    {permissionMode ? (
      <div className="min-w-0 basis-full [overflow-wrap:anywhere]">
        <dt className="inline">{t("contacts.metadata.permission")}</dt>{" "}
        <dd className="inline">{permissionMode}</dd>
      </div>
    ) : null}
    {provider ? (
      <div className="min-w-0 max-w-full [overflow-wrap:anywhere]">
        <dt className="inline">{t("contacts.metadata.provider")}</dt>{" "}
        <dd className="inline">{provider}</dd>
      </div>
    ) : null}
  </dl>;
}

function ContactsAgentActions({ agent, chatLabel, onOpenRoom, compact = false }:
  Pick<ContactsAgentCardViewProps, "agent" | "chatLabel" | "onOpenRoom"> & { compact?: boolean }) {
  const actions = [
    { label: chatLabel, command: onOpenRoom, Icon: MessageCircle, tone: "default" as const },
  ];
  return <div className={cn("flex flex-wrap items-center", compact ? "gap-1" : "gap-x-3 gap-y-1")}>
    {actions.map(({ label, command, Icon, tone }) => compact ? (
      <UiIconButton aria-label={`${label} ${agent.name}`} key={label} onClick={(event) => {
        event.stopPropagation();
        command();
      }} size="sm" title={label} tone={tone}>
        <Icon aria-hidden className="h-4 w-4" />
      </UiIconButton>
    ) : (
      <WorkspaceCatalogTextAction aria-label={`${label} ${agent.name}`} key={label} onClick={command} tone={tone}>
        <Icon aria-hidden className="h-4 w-4" />{label}
      </WorkspaceCatalogTextAction>
    ))}
  </div>;
}

function ContactsAgentBusinessTags({
  className,
  tags,
}: {
  className?: string;
  tags: string[];
}) {
  const visibleTags = tags.slice(0, 2);
  if (visibleTags.length === 0) {
    return null;
  }
  return (
    <div className={cn("flex min-w-0 flex-wrap items-center gap-1", className)}>
      {visibleTags.map((tag) => (
        <UiBadge
          className="max-w-full whitespace-normal [overflow-wrap:anywhere]"
          key={tag}
          shape="pill"
          size="sm"
          title={tag}
          tone="idle"
        >
          {tag}
        </UiBadge>
      ))}
      {tags.length > visibleTags.length ? (
        <span className={cn(
          "shrink-0",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
        )}>
          +{tags.length - visibleTags.length}
        </span>
      ) : null}
    </div>
  );
}
