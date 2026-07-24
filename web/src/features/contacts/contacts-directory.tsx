"use client";

import { Plus, Users } from "lucide-react";
import { useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/controls/workspace-search-input";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceCatalogGhostAction } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/catalog/workspace-icon-frame";
import { Agent } from "@/types/agent/agent";

import { ContactsAgentCard } from "./contacts-agent-card";
import { matchesContactsSearch } from "./contacts-directory-helpers";

interface ContactsDirectoryProps {
  agents: Agent[];
  onOpenDirectRoom: (agentId: string) => void;
  onCreateAgent: () => void;
  onEditAgent: (agentId: string) => void;
  onCreateTeam: (agentId: string) => void;
}

export function ContactsDirectory({
  agents,
  onOpenDirectRoom,
  onCreateAgent,
  onEditAgent,
  onCreateTeam,
}: ContactsDirectoryProps) {
  const { t } = useI18n();
  const [searchQuery, setSearchQuery] = useState("");

  const filteredAgents = useMemo(
    () => agents.filter((agent) => matchesContactsSearch(agent, searchQuery)),
    [agents, searchQuery],
  );

  const headerTrailing = (
    <WorkspaceSearchInput
      className="hidden sm:inline-flex"
      inputClassName="w-[200px]"
      onChange={setSearchQuery}
      placeholder={t("common.search_agents")}
      value={searchQuery}
    />
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        leading={<Users className="h-4 w-4 text-(--icon-default)" />}
        title={t("contacts.title")}
        trailing={headerTrailing}
      />

      <div className="soft-scrollbar scrollbar-stable-gutter min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-5 sm:py-5 xl:px-6">
        <div className="grid grid-cols-1 gap-3.5 md:grid-cols-2 md:gap-4 xl:grid-cols-3 2xl:grid-cols-4">
          <button
            className="flex min-h-[138px] w-full items-center gap-3 rounded-[12px] border border-dashed border-(--divider-subtle-color) bg-transparent px-4 py-4 text-left transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background) md:hidden"
            onClick={onCreateAgent}
            type="button"
          >
            <WorkspaceIconFrame className="h-10 w-10 shrink-0" shape="round" size="md">
              <Plus className="h-4.5 w-4.5 text-(--icon-default)" />
            </WorkspaceIconFrame>
            <span className="min-w-0">
              <span className="block truncate text-[15px] font-semibold text-(--text-strong)">
                {t("contacts.new_agent")}
              </span>
              <span className="mt-1 block line-clamp-2 text-[11px] leading-5 text-(--text-muted)">
                {t("contacts.new_agent_description")}
              </span>
            </span>
          </button>
          <WorkspaceCatalogGhostAction
            className="hidden py-8 md:flex"
            onClick={onCreateAgent}
            size="comfort"
          >
            <WorkspaceIconFrame className="h-16 w-16" shape="round" size="lg">
              <Plus className="h-7 w-7 text-(--icon-default)" />
            </WorkspaceIconFrame>
            <p className="mt-4 text-[18px] font-bold tracking-[-0.03em] text-(--text-strong)">
              {t("contacts.new_agent")}
            </p>
            <p className="mt-2 text-[13px] leading-5 text-(--text-default)">
              {t("contacts.new_agent_description")}
            </p>
          </WorkspaceCatalogGhostAction>
          {filteredAgents.map((agent) => (
            <ContactsAgentCard
              key={agent.agent_id}
              agent={agent}
              onCreateTeam={() => onCreateTeam(agent.agent_id)}
              onOpenProfile={() => onEditAgent(agent.agent_id)}
              onOpenRoom={() => onOpenDirectRoom(agent.agent_id)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
