/**
 * INPUT: Agent 身份与打开详情、私聊、群聊的页面命令。
 * OUTPUT: 窄屏摘要卡、桌面完整卡与高密度列表行。
 * POS: 联系人管理目录卡片；默认层承担 Agent 选择所需的比较信息。
 */
"use client";

import { MessageCirclePlus, MessageSquareText } from "lucide-react";

import { AGENT_PERMISSION_MODES } from "@/lib/agent-options";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
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
  onCreateTeam: () => void;
  view: "grid" | "list";
}

interface ContactsAgentCardViewProps extends Omit<ContactsAgentCardProps, "view"> {
  allowedToolsCount: number;
  businessTags: string[];
  chatLabel: string;
  createTeamLabel: string;
  editLabel: string;
  permissionMode: string;
  provider: string;
  skillsCount: number;
}

export function ContactsAgentCard({
  agent,
  onOpenProfile: onOpenProfile,
  onOpenRoom: onOpenRoom,
  onCreateTeam: onCreateTeam,
  view,
}: ContactsAgentCardProps) {
  const { t } = useI18n();

  const permissionMode = AGENT_PERMISSION_MODES.find(
    (option) => option.value === agent.options.permission_mode,
  ) ?? AGENT_PERMISSION_MODES[0];
  const provider = agent.options.provider?.trim()
    ? formatProviderLabel(agent.options.provider)
    : t("agent_options.identity.follow_default_provider");
  const allowedToolsCount = agent.options.allowed_tools?.length || 0;
  const skillsCount = agent.skills_count || 0;
  const businessTags = [...new Set(
    (agent.business_tags ?? []).map((tag) => tag.trim()).filter(Boolean),
  )];

  const viewProps: ContactsAgentCardViewProps = {
    agent,
    allowedToolsCount,
    businessTags,
    chatLabel: t("contacts.chat"),
    createTeamLabel: t("contacts.create_team"),
    editLabel: t("common.edit"),
    onCreateTeam,
    onOpenProfile,
    onOpenRoom,
    permissionMode: t(permissionMode.labelKey),
    provider,
    skillsCount,
  };

  if (view === "list") {
    return <ContactsAgentListRow {...viewProps} />;
  }

  return (
    <>
      <ContactsAgentCompactCard {...viewProps} />
      <ContactsAgentComfortCard {...viewProps} />
    </>
  );
}

function ContactsAgentListRow({
  agent,
  allowedToolsCount,
  businessTags,
  chatLabel,
  createTeamLabel,
  onCreateTeam,
  onOpenProfile,
  onOpenRoom,
  permissionMode,
  provider,
  skillsCount,
}: ContactsAgentCardViewProps) {
  const { t } = useI18n();

  return (
    <UiListRow
      className="rounded-none"
      leading={<UiAgentAvatar avatar={agent.avatar} name={agent.name} size="md" />}
      onClick={onOpenProfile}
      right={(
        <div className="flex shrink-0 items-center gap-1">
          <WorkspaceCatalogTextAction
            aria-label={chatLabel}
            onClick={(event) => {
              event.stopPropagation();
              onOpenRoom();
            }}
            tone="primary"
          >
            <MessageSquareText className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">{chatLabel}</span>
          </WorkspaceCatalogTextAction>
          <WorkspaceCatalogTextAction
            aria-label={createTeamLabel}
            onClick={(event) => {
              event.stopPropagation();
              onCreateTeam();
            }}
          >
            <MessageCirclePlus className="h-3.5 w-3.5" />
            <span className="hidden lg:inline">{createTeamLabel}</span>
          </WorkspaceCatalogTextAction>
        </div>
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <h3 className={cn(
            "truncate",
            getUiTypographyClassName({
              role: "sectionTitle",
              tone: "strong",
            }),
          )}>
            {agent.name}
          </h3>
          <UiBadge className="max-w-[128px] truncate" size="xs" tone="idle">
            {permissionMode}
          </UiBadge>
          <ContactsAgentBusinessTags className="hidden md:flex" tags={businessTags} />
          <span className={cn(
            "min-w-0 flex-1 truncate",
            getUiTypographyClassName({ role: "caption", tone: "soft" }),
          )}>
            {t("contacts.metadata.provider")}: {provider}
            {" · "}
            {t("contacts.metadata.tools")} {allowedToolsCount}
            {" · "}
            {t("contacts.metadata.skills")} {skillsCount}
          </span>
        </div>
        <p className={cn(
          "mt-0.5 truncate",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
        )}>
          {agent.description || t("contacts.no_description")}
        </p>
      </div>
    </UiListRow>
  );
}

function ContactsAgentCompactCard({
  agent,
  allowedToolsCount,
  businessTags,
  chatLabel,
  createTeamLabel,
  editLabel,
  onCreateTeam,
  onOpenProfile,
  onOpenRoom,
  permissionMode,
  provider,
  skillsCount,
}: ContactsAgentCardViewProps) {
  const { t } = useI18n();

  return (
    <WorkspaceCatalogCard
      align="start"
      className="group relative h-full overflow-hidden hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background) md:hidden"
      size="compact"
    >
      <UiButton
        aria-label={`${editLabel} ${agent.name}`}
        className="surface-radius-md absolute inset-0 z-0 h-full min-h-0 w-full border-0 p-0 focus-visible:ring-inset"
        onClick={onOpenProfile}
        variant="ghost"
      >
        <span className="sr-only">{`${editLabel} ${agent.name}`}</span>
      </UiButton>

      <div className="pointer-events-none relative z-10 flex w-full min-w-0 items-start gap-3">
        <UiAgentAvatar
          avatar={agent.avatar}
          name={agent.name}
          size="md"
        />

        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <WorkspaceCatalogTitle className="min-w-0 flex-1" size="sm" truncate>
              {agent.name}
            </WorkspaceCatalogTitle>
            <UiBadge className="max-w-[112px] truncate" size="xs" tone="idle">
              {permissionMode}
            </UiBadge>
          </div>

          {agent.description && (
            <WorkspaceCatalogDescription className="mt-1" lines={1}>
              {agent.description}
            </WorkspaceCatalogDescription>
          )}

          <ContactsAgentBusinessTags className="mt-1.5" tags={businessTags} />

          <div className={cn(
            "mt-2 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1",
            getUiTypographyClassName({ role: "caption", tone: "soft" }),
          )}>
            <span className="min-w-0 max-w-full truncate">
              <span className="text-(--text-default)">{t("contacts.metadata.provider")}</span>
              {" · "}
              {provider}
            </span>
            <span>{t("contacts.metadata.tools")} {allowedToolsCount}</span>
            <span>{t("contacts.metadata.skills")} {skillsCount}</span>
          </div>
        </div>
      </div>

      <WorkspaceCatalogFooter
        className="relative z-20 mt-3 w-full gap-4 border-t border-(--divider-subtle-color) pt-2.5"
        justify="start"
      >
        <WorkspaceCatalogTextAction onClick={onOpenRoom} tone="primary">
          <MessageSquareText className="h-3 w-3" />
          {chatLabel}
        </WorkspaceCatalogTextAction>
        <WorkspaceCatalogTextAction onClick={onCreateTeam}>
          <MessageCirclePlus className="h-3 w-3" />
          {createTeamLabel}
        </WorkspaceCatalogTextAction>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}

function ContactsAgentComfortCard({
  agent,
  allowedToolsCount,
  businessTags,
  chatLabel,
  createTeamLabel,
  editLabel,
  onCreateTeam,
  onOpenProfile,
  onOpenRoom,
  permissionMode,
  provider,
  skillsCount,
}: ContactsAgentCardViewProps) {
  const { t } = useI18n();

  return (
    <WorkspaceCatalogCard
      align="center"
      className="group relative hidden h-full overflow-hidden hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background) md:flex"
      size="comfort"
    >
      <UiButton
        aria-label={`${editLabel} ${agent.name}`}
        className="surface-radius-lg absolute inset-0 z-0 h-full min-h-0 w-full border-0 p-0 focus-visible:ring-inset"
        onClick={onOpenProfile}
        variant="ghost"
      >
        <span className="sr-only">{`${editLabel} ${agent.name}`}</span>
      </UiButton>

      <div className="pointer-events-none relative z-10 flex w-full flex-col items-center">
        <UiAgentAvatar
          avatar={agent.avatar}
          className="mx-auto"
          name={agent.name}
          size="lg"
        />

        <WorkspaceCatalogBody className="mt-3 w-full" grow={false}>
          <WorkspaceCatalogTitle size="lg" truncate>
            {agent.name}
          </WorkspaceCatalogTitle>

          {agent.description && (
            <WorkspaceCatalogDescription
              className="mt-1.5"
              minHeight={false}
            >
              {agent.description}
            </WorkspaceCatalogDescription>
          )}
          <ContactsAgentBusinessTags
            className="mt-2 justify-center"
            tags={businessTags}
          />

          <div className={cn(
            "mt-2 flex flex-col items-center justify-center gap-1 text-center",
            getUiTypographyClassName({ role: "caption", tone: "soft" }),
          )}>
            <div className="flex flex-wrap gap-1.5">
              <span className="text-(--text-default)">{t("contacts.metadata.permission")}:</span>
              <span className="text-(--text-muted)">{permissionMode}</span>
            </div>
            <div className="flex flex-wrap items-center justify-center gap-1.5">
              <span className="text-(--text-default)">{t("contacts.metadata.provider")}:</span>
              <span className="text-(--text-muted)">{provider}</span>
              <span className="mx-0.5">•</span>
              <span className="text-(--text-default)">{t("contacts.metadata.tools")}:</span>
              <span className="text-(--text-muted)">{allowedToolsCount}</span>
              <span className="mx-0.5">•</span>
              <span className="text-(--text-default)">{t("contacts.metadata.skills")}:</span>
              <span className="text-(--text-muted)">{skillsCount}</span>
            </div>
          </div>
        </WorkspaceCatalogBody>
      </div>

      <WorkspaceCatalogFooter className="relative z-20 mt-2 w-full gap-4" justify="center">
        <WorkspaceCatalogTextAction onClick={onOpenRoom} tone="primary">
          <MessageSquareText className="h-3 w-3" />
          {chatLabel}
        </WorkspaceCatalogTextAction>
        <WorkspaceCatalogTextAction onClick={onCreateTeam}>
          <MessageCirclePlus className="h-3 w-3" />
          {createTeamLabel}
        </WorkspaceCatalogTextAction>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
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
    <div className={cn("flex min-w-0 items-center gap-1", className)}>
      {visibleTags.map((tag) => (
        <UiBadge
          className="max-w-[140px] truncate"
          key={tag}
          shape="pill"
          size="xs"
          title={tag}
          tone="idle"
        >
          {tag}
        </UiBadge>
      ))}
      {tags.length > visibleTags.length ? (
        <span className={cn(
          "shrink-0",
          getUiTypographyClassName({ role: "caption", tone: "soft" }),
        )}>
          +{tags.length - visibleTags.length}
        </span>
      ) : null}
    </div>
  );
}
