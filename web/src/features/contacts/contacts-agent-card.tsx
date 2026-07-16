"use client";

import { MessageSquareText, Users } from "lucide-react";

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

export function ContactsAgentCard({
  agent,
  onOpenProfile: onOpenProfile,
  onOpenRoom: onOpenRoom,
  onCreateTeam: onCreateTeam,
}: ContactsAgentCardProps) {
  const { t } = useI18n();

  const permissionMode = agent.options.permission_mode || "default";
  const provider = formatProviderLabel(agent.options.provider);
  const allowedToolsCount = agent.options.allowed_tools?.length || 0;
  const skillsCount = agent.skills_count || 0;

  return (
    <WorkspaceCatalogCard
      align="center"
      className="group relative h-full overflow-hidden hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)"
      size="comfort"
    >
      <button
        aria-label={`${t("common.edit")} ${agent.name}`}
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
            <WorkspaceCatalogDescription className="mt-1.5 line-clamp-2 text-[13px] leading-tight" minHeight={false}>
              {agent.description}
            </WorkspaceCatalogDescription>
          )}

          <div className="mt-2 flex flex-col gap-1 text-[11px] text-(--text-soft) items-center justify-center text-center">
            <div className="flex flex-wrap gap-1.5">
              <span className="text-(--text-default)">权限:</span>
              <span className="text-(--text-muted)">{permissionMode}</span>
            </div>
            <div className="flex flex-wrap gap-1.5 items-center justify-center">
              <span className="text-(--text-default)">Provider:</span>
              <span className="text-(--text-muted)">{provider}</span>
              <span className="mx-0.5">•</span>
              <span className="text-(--text-default)">工具:</span>
              <span className="text-(--text-muted)">{allowedToolsCount}</span>
              <span className="mx-0.5">•</span>
              <span className="text-(--text-default)">Skill:</span>
              <span className="text-(--text-muted)">{skillsCount}</span>
            </div>
          </div>
        </WorkspaceCatalogBody>
      </div>

      <WorkspaceCatalogFooter className="relative z-20 mt-2 w-full gap-4" justify="center">
        <WorkspaceCatalogTextAction onClick={onOpenRoom} tone="primary">
          <MessageSquareText className="h-3 w-3" />
          {t("contacts.chat")}
        </WorkspaceCatalogTextAction>
        <WorkspaceCatalogTextAction onClick={onCreateTeam}>
          <Users className="h-3 w-3" />
          {t("contacts.create_team")}
        </WorkspaceCatalogTextAction>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
