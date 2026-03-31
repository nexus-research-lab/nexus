"use client";

import { Plus, Users } from "lucide-react";
import { useMemo, useState } from "react";

import { WorkspaceSearchInput } from "@/shared/ui/workspace/workspace-search-input";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/workspace-surface-header";
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
    {key: "my_agents", label: "My Agents"},
    {key: "task_generated", label: "Task Generated"},
  ];

  // Header 右侧：搜索框
  const header_trailing = (
    <WorkspaceSearchInput
      class_name="hidden sm:inline-flex"
      input_class_name="w-[200px]"
      on_change={set_search_query}
      placeholder="搜索成员..."
      value={search_query}
    />
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        active_tab={active_tab}
        badge="AGENTS"
        leading={<Users className="h-4 w-4 text-slate-800/72"/>}
        on_change_tab={set_active_tab}
        subtitle="管理你的 AI 成员，配置身份与能力"
        tabs={tabs}
        title="Agents"
        trailing={header_trailing}
      />

      {/* 卡片网格区域 */}
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
        <div className="grid grid-cols-3 gap-6 md:grid-cols-3 xl:grid-cols-4">
          {/* 首张卡片 — New Agent */}
          {active_tab != "task_generated" && (<article
              className="workspace-card flex cursor-pointer flex-col items-center justify-center rounded-[26px] border border-dashed border-slate-300/40 px-6 py-8 text-center transition-all hover:border-slate-400/50 hover:bg-white/34"
              onClick={on_create_agent}
            >
              <div
                className="flex h-16 w-16 items-center justify-center rounded-full border border-white/44 bg-white/64">
                <Plus className="h-7 w-7 text-slate-600"/>
              </div>
              <p className="mt-4 text-[18px] font-bold tracking-[-0.03em] text-slate-950/80">
                New Agent
              </p>
              <p className="mt-2 text-[13px] leading-5 text-slate-700/60">
                创建一个新的 AI 成员
              </p>
            </article>
          )}

          {/* Agent 卡片列表 */}
          {filtered_agents.map((agent) => (
            <ContactsAgentCard
              key={agent.agent_id}
              description={get_contacts_agent_description(agent)}
              name={agent.name}
              on_create_team={() => on_create_team(agent.agent_id)}
              on_open_profile={() => on_edit_agent(agent.agent_id)}
              on_open_room={() => on_open_direct_room(agent.agent_id)}
            />
          ))}
        </div>

        {/* Task Generated 空状态 */}
        {active_tab === "task_generated" && (
          <div
            className="workspace-card mt-6 flex min-h-80 items-center justify-center rounded-[28px] px-8 text-center">
            <div>
              <p className="text-[22px] font-bold tracking-[-0.04em] text-slate-950/90">
                暂无任务生成的成员
              </p>
              <p className="mt-3 text-sm leading-7 text-slate-700/60">
                任务自动创建的 Agent 将显示在这里。
              </p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
