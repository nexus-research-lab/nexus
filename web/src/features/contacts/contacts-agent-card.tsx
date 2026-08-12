"use client";

import { MessageCirclePlus, MessageSquareText } from "lucide-react";

import { AGENT_PERMISSION_MODES } from "@/lib/agent-options";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
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
}

interface ContactsAgentCardViewProps extends ContactsAgentCardProps {
  editLabel: string;
  chatLabel: string;
  createTeamLabel: string;
  permissionMode: string;
  provider: string;
  allowedToolsCount: number;
  skillsCount: number;
}

export function ContactsAgentCard({
  agent,
  onOpenProfile: onOpenProfile,
  onOpenRoom: onOpenRoom,
  onCreateTeam: onCreateTeam,
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

  return (
    <>
      <ContactsAgentCompactCard
        agent={agent}
        allowedToolsCount={allowedToolsCount}
        chatLabel={t("contacts.chat")}
        createTeamLabel={t("contacts.create_team")}
        editLabel={t("common.edit")}
        onCreateTeam={onCreateTeam}
        onOpenProfile={onOpenProfile}
        onOpenRoom={onOpenRoom}
        permissionMode={t(permissionMode.labelKey)}
        provider={provider}
        skillsCount={skillsCount}
      />
      <ContactsAgentComfortCard
        agent={agent}
        allowedToolsCount={allowedToolsCount}
        chatLabel={t("contacts.chat")}
        createTeamLabel={t("contacts.create_team")}
        editLabel={t("common.edit")}
        onCreateTeam={onCreateTeam}
        onOpenProfile={onOpenProfile}
        onOpenRoom={onOpenRoom}
        permissionMode={t(permissionMode.labelKey)}
        provider={provider}
        skillsCount={skillsCount}
      />
    </>
  );
}

function ContactsAgentCompactCard({
  agent,
  allowedToolsCount,
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
      <button
        aria-label={`${editLabel} ${agent.name}`}
        className="absolute inset-0 z-0 rounded-[inherit] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary/40"
        onClick={onOpenProfile}
        type="button"
      />

      <div className="pointer-events-none relative z-10 flex w-full min-w-0 items-start gap-3">
        <UiAgentAvatar
          avatar={agent.avatar}
          name={agent.name}
          size="md"
        />

        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <h3 className="min-w-0 flex-1 truncate text-[16px] font-semibold text-(--text-strong)">
              {agent.name}
            </h3>
            <span className="inline-flex max-w-[112px] shrink-0 truncate rounded-[6px] border border-(--divider-subtle-color) px-1.5 py-0.5 text-2xs font-medium text-(--text-soft)">
              {permissionMode}
            </span>
          </div>

          {agent.description && (
            <p className="mt-1 line-clamp-1 text-xs leading-5 text-(--text-muted)">
              {agent.description}
            </p>
          )}

          <div className="mt-2 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-(--text-soft)">
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
      <button
        aria-label={`${editLabel} ${agent.name}`}
        className="absolute inset-0 z-0 rounded-[inherit] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary/40"
        onClick={onOpenProfile}
        type="button"
      />

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
              className="mt-1.5 line-clamp-2 text-sm leading-tight"
              minHeight={false}
            >
              {agent.description}
            </WorkspaceCatalogDescription>
          )}

          <div className="mt-2 flex flex-col items-center justify-center gap-1 text-center text-xs text-(--text-soft)">
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
