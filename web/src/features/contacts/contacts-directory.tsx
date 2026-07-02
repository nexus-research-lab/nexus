"use client";

import { Plus, Users } from "lucide-react";
import { useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/controls/workspace-search-input";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import {
  WorkspaceCatalogGhostCard,
  WorkspaceIconFrame,
} from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import { Agent } from "@/types/agent/agent";

import { ContactsAgentCard } from "./contacts-agent-card";
import { matches_contacts_search } from "./contacts-directory-helpers";

interface ContactsDirectoryProps {
  agents: Agent[];
  /** 💬 Chat → ensureDirectRoom 发起 DM */
  on_open_direct_room: (agentId: string) => void;
  /** 新建 Agent → 打开 AgentOptions 对话框（create 模式） */
  on_create_agent: () => void;
  /** 点击卡片 → 打开 AgentOptions 对话框（edit 模式） */
  on_edit_agent: (agentId: string) => void;
  /** 👥 Create Team → 用该 Agent 创建 Room */
  on_create_team: (agentId: string) => void;
}

/** Contacts 全宽卡片网格 — 风格 */
export function ContactsDirectory({
  agents,
  on_open_direct_room: onOpenDirectRoom,
  on_create_agent: onCreateAgent,
  on_edit_agent: onEditAgent,
  on_create_team: onCreateTeam,
}: ContactsDirectoryProps) {
  const { t } = useI18n();
  const [searchQuery, setSearchQuery] = useState("");

  // Tab 过滤 + 搜索
  const filteredAgents = useMemo(() => {
    return agents.filter((agent) => {
      return matches_contacts_search(agent, searchQuery);
    });
  }, [agents, searchQuery]);

  // Header 右侧：搜索框
  const headerTrailing = (
    <WorkspaceSearchInput
      class_name="hidden sm:inline-flex"
      input_class_name="w-[200px]"
      on_change={setSearchQuery}
      placeholder={t("common.search_agents")}
      value={searchQuery}
    />
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        badge="AGENTS"
        density="compact"
        leading={<Users className="h-4 w-4 text-(--icon-default)" />}
        title={t("contacts.title")}
        trailing={headerTrailing}
      />

      {/* 卡片网格区域 */}
      <div className="soft-scrollbar scrollbar-stable-gutter min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {/* 首张卡片 — New Agent */}
          <WorkspaceCatalogGhostCard
            class_name="py-8"
            onClick={onCreateAgent}
            size="comfort"
          >
            <WorkspaceIconFrame class_name="h-16 w-16" shape="round" size="lg">
              <Plus className="h-7 w-7 text-(--icon-default)" />
            </WorkspaceIconFrame>
            <p className="mt-4 text-[18px] font-bold tracking-[-0.03em] text-(--text-strong)">
              {t("contacts.new_agent")}
            </p>
            <p className="mt-2 text-[13px] leading-5 text-(--text-default)">
              {t("contacts.new_agent_description")}
            </p>
          </WorkspaceCatalogGhostCard>

          {/* Agent 卡片列表 */}
          {filteredAgents.map((agent) => (
            <ContactsAgentCard
              key={agent.agent_id}
              agent={agent}
              on_create_team={() => onCreateTeam(agent.agent_id)}
              on_open_profile={() => onEditAgent(agent.agent_id)}
              on_open_room={() => onOpenDirectRoom(agent.agent_id)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
