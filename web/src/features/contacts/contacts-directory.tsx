/**
 * INPUT: Agent 目录与创建、详情、私聊、群聊导航命令。
 * OUTPUT: 可搜索的响应式 Agent 管理目录，窄屏摘要、桌面完整比较。
 * POS: 联系人正文根目录；承载选择 Agent 所需的识别和能力概况。
 */
"use client";

import { Plus } from "lucide-react";
import { useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import {
  WORKSPACE_CATALOG_GRID_CLASS_NAME,
  WORKSPACE_CONTENT_PAGE_CLASS_NAME,
} from "@/shared/ui/layout/workspace-content-layout";
import { WorkspaceCatalogGhostAction } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/catalog/workspace-icon-frame";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/controls/workspace-search-input";
import { Agent } from "@/types/agent/agent";

import { ContactsAgentCard } from "./contacts-agent-card";
import { matchesContactsSearch } from "./contacts-directory-helpers";

interface ContactsDirectoryProps {
  agents: Agent[];
  onOpenDirectRoom: (agentId: string) => void;
  onCreateAgent: () => void;
  onOpenAgent: (agentId: string) => void;
  onCreateTeam: (agentId: string) => void;
}

export function ContactsDirectory({
  agents,
  onOpenDirectRoom,
  onCreateAgent,
  onOpenAgent,
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
      <div className="soft-scrollbar scrollbar-stable-gutter min-h-0 flex-1 overflow-y-auto">
        <div className={WORKSPACE_CONTENT_PAGE_CLASS_NAME}>
          <WorkspaceContentHeader
            actions={headerTrailing}
            description={t("contacts.description")}
            title={t("contacts.title")}
          />
          <div className={cn(
            WORKSPACE_CATALOG_GRID_CLASS_NAME,
            "gap-3.5 md:gap-4",
          )}>
            <button
              className="flex min-h-[138px] w-full items-center gap-3 rounded-[12px] border border-dashed border-(--divider-subtle-color) bg-transparent px-4 py-4 text-left transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background) md:hidden"
              onClick={onCreateAgent}
              type="button"
            >
              <WorkspaceIconFrame className="h-10 w-10 shrink-0" shape="round" size="md">
                <Plus className="h-4.5 w-4.5 text-(--icon-default)" />
              </WorkspaceIconFrame>
              <span className="min-w-0">
                <span className="block truncate text-base font-semibold text-(--text-strong)">
                  {t("contacts.new_agent")}
                </span>
                <span className="mt-1 block line-clamp-2 text-xs leading-5 text-(--text-muted)">
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
              <p className="mt-4 text-md font-semibold tracking-[-0.03em] text-(--text-strong)">
                {t("contacts.new_agent")}
              </p>
              <p className="mt-2 text-sm leading-5 text-(--text-default)">
                {t("contacts.new_agent_description")}
              </p>
            </WorkspaceCatalogGhostAction>
            {filteredAgents.map((agent) => (
              <ContactsAgentCard
                key={agent.agent_id}
                agent={agent}
                onCreateTeam={() => onCreateTeam(agent.agent_id)}
                onOpenProfile={() => onOpenAgent(agent.agent_id)}
                onOpenRoom={() => onOpenDirectRoom(agent.agent_id)}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
