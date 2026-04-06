"use client";

import { Plus, Users } from "lucide-react";
import { useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/workspace-search-input";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/workspace-surface-header";
import {
  WorkspaceCatalogEmptyShell,
  WorkspaceCatalogGhostCard,
  WorkspaceIconFrame,
} from "@/shared/ui/workspace/workspace-catalog-card";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";

import { ContactsAgentCard } from "./contacts-agent-card";
import {
  get_contacts_agent_conversations,
  get_contacts_agent_description,
  matches_contacts_search,
} from "./contacts-directory-helpers";

/** Tab 过滤键 — My Agents / Task Generated */
type ContactsTabKey = "my_agents" | "task_generated";

interface ContactsDirectoryProps {
  agents: Agent[];
  conversations: Conversation[];
  /** 💬 Chat → ensureDirectRoom 发起 DM */
  on_open_direct_room: (agent_id: string) => void;
  /** 新建 Agent → 打开 AgentOptions 对话框（create 模式） */
  on_create_agent: () => void;
  /** 点击卡片 → 打开 AgentOptions 对话框（edit 模式） */
  on_edit_agent: (agent_id: string) => void;
  /** 👥 Create Team → 用该 Agent 创建 Room */
  on_create_team: (agent_id: string) => void;
}

/** Contacts 全宽卡片网格 — 风格 */
export function ContactsDirectory({
                                    agents,
                                    conversations,
                                    on_open_direct_room,
                                    on_create_agent,
                                    on_edit_agent,
                                    on_create_team,
                                  }: ContactsDirectoryProps) {
  const { t } = useI18n();
  const [active_tab, set_active_tab] = useState<ContactsTabKey>("my_agents");
  const [search_query, set_search_query] = useState("");

  // 按 agent 分组 conversations（用于后续扩展）
  const _conversations_by_agent = useMemo(() => {
    const grouped = new Map<string, Conversation[]>();
    agents.forEach((agent) => {
      grouped.set(agent.agent_id, get_contacts_agent_conversations(conversations, agent.agent_id));
    });
    return grouped;
  }, [agents, conversations]);

  // Tab 过滤 + 搜索
  const filtered_agents = useMemo(() => {
    return agents.filter((agent) => {
      // Tab 过滤：task_generated 暂时为空（后续接入）
      if (active_tab === "task_generated") return false;
      return matches_contacts_search(agent, search_query);
    });
  }, [active_tab, agents, search_query]);

  // Header tabs 定义
  const tabs: { key: ContactsTabKey; label: string }[] = [
    {key: "my_agents", label: t("contacts.tab_my_agents")},
    {key: "task_generated", label: t("contacts.tab_task_generated")},
  ];

  // Header 右侧：搜索框
  const header_trailing = (
    <WorkspaceSearchInput
      class_name="hidden sm:inline-flex"
      input_class_name="w-[200px]"
      on_change={set_search_query}
      placeholder={t("common.search_agents")}
      value={search_query}
    />
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        active_tab={active_tab}
        badge="AGENTS"
        density="compact"
        leading={<Users className="h-4 w-4 text-slate-800/72"/>}
        on_change_tab={set_active_tab}
        subtitle={t("contacts.subtitle")}
        tabs={tabs}
        title={t("contacts.title")}
        trailing={header_trailing}
      />

      {/* 卡片网格区域 */}
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
        <div className="grid grid-cols-3 gap-6 md:grid-cols-3 xl:grid-cols-4">
          {/* 首张卡片 — New Agent */}
          {active_tab != "task_generated" && (
            <WorkspaceCatalogGhostCard
              class_name="px-6 py-8"
              onClick={on_create_agent}
            >
              <WorkspaceIconFrame class_name="h-16 w-16" shape="round" size="lg">
                <Plus className="h-7 w-7 text-slate-600"/>
              </WorkspaceIconFrame>
              <p className="mt-4 text-[18px] font-bold tracking-[-0.03em] text-slate-950/92">
                {t("contacts.new_agent")}
              </p>
              <p className="mt-2 text-[13px] leading-5 text-slate-700/72">
                {t("contacts.new_agent_description")}
              </p>
            </WorkspaceCatalogGhostCard>
          )}

          {/* Agent 卡片列表 */}
          {filtered_agents.map((agent) => (
            <ContactsAgentCard
              key={agent.agent_id}
              description={get_contacts_agent_description(agent, t("contacts.default_description"))}
              name={agent.name}
              on_create_team={() => on_create_team(agent.agent_id)}
              on_open_profile={() => on_edit_agent(agent.agent_id)}
              on_open_room={() => on_open_direct_room(agent.agent_id)}
            />
          ))}
        </div>

        {/* Task Generated 空状态 */}
        {active_tab === "task_generated" && (
          <WorkspaceCatalogEmptyShell class_name="mt-6">
            <div>
              <p className="text-[22px] font-bold tracking-[-0.04em] text-slate-950/90">
                {t("contacts.empty_generated_title")}
              </p>
              <p className="mt-3 text-sm leading-7 text-slate-700/60">
                {t("contacts.empty_generated_description")}
              </p>
            </div>
          </WorkspaceCatalogEmptyShell>
        )}
      </div>
    </div>
  );
}
