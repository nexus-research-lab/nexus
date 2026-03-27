"use client";

import {
  Grid2X2,
  List,
  Users,
} from "lucide-react";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { WorkspaceCanvasShell } from "@/shared/ui/workspace-canvas-shell";
import { WorkspaceIconToggleButton } from "@/shared/ui/workspace-icon-toggle-button";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { WorkspaceSearchInput } from "@/shared/ui/workspace-search-input";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";

import { ContactsAgentCard } from "./contacts-agent-card";
import {
  ContactsFilterKey,
  get_contacts_agent_conversations,
  get_contacts_agent_description,
  get_contacts_model_label,
  get_contacts_runtime_label,
  get_contacts_runtime_status,
  get_contacts_runtime_tone,
  matches_contacts_filter,
  matches_contacts_search,
} from "./contacts-directory-helpers";
import { ContactsFilterSidebar } from "./contacts-filter-sidebar";
import { ContactsProfilePanel } from "./contacts-profile-panel";

interface ContactsDirectoryProps {
  agents: Agent[];
  conversations: Conversation[];
  on_open_direct_room: (agent_id: string) => void;
  on_create_agent: () => void;
  on_edit_agent: (agent_id: string) => void;
  on_delete_agent: (agent_id: string) => void;
  selected_agent_id?: string;
}

export function ContactsDirectory({
  agents,
  conversations,
  on_open_direct_room,
  on_create_agent,
  on_edit_agent,
  on_delete_agent,
  selected_agent_id,
}: ContactsDirectoryProps) {
  const navigate = useNavigate();
  const [active_filter, set_active_filter] = useState<ContactsFilterKey>("all");
  const [search_query, set_search_query] = useState("");
  const [view_mode, set_view_mode] = useState<"grid" | "list">("grid");

  const conversations_by_agent = useMemo(() => {
    const grouped = new Map<string, Conversation[]>();
    agents.forEach((agent) => {
      grouped.set(agent.agent_id, get_contacts_agent_conversations(conversations, agent.agent_id));
    });
    return grouped;
  }, [agents, conversations]);

  const selected_agent =
    agents.find((agent) => agent.agent_id === selected_agent_id) ?? agents[0] ?? null;

  const filtered_agents = useMemo(() => (
    agents.filter((agent) => {
      const agent_conversations = conversations_by_agent.get(agent.agent_id) ?? [];
      return (
        matches_contacts_filter(agent, agent_conversations, active_filter) &&
        matches_contacts_search(agent, search_query)
      );
    })
  ), [active_filter, agents, conversations_by_agent, search_query]);

  const selected_agent_conversations = selected_agent
    ? conversations_by_agent.get(selected_agent.agent_id) ?? []
    : [];

  const filter_sections = useMemo(() => {
    const build_count = (filter: ContactsFilterKey) => agents.filter((agent) => (
      matches_contacts_filter(agent, conversations_by_agent.get(agent.agent_id) ?? [], filter)
    )).length;

    return [
      {
        title: "Main Directory",
        items: [
          { key: "all" as const, label: "All Agents", count: build_count("all") },
          { key: "recent" as const, label: "Recent", count: build_count("recent") },
        ],
      },
      {
        title: "By Status",
        items: [
          { key: "running" as const, label: "Running", count: build_count("running"), dot_class_name: "bg-emerald-300" },
          { key: "active" as const, label: "Active", count: build_count("active"), dot_class_name: "bg-sky-300" },
          { key: "idle" as const, label: "Idle", count: build_count("idle"), dot_class_name: "bg-slate-400" },
        ],
      },
      {
        title: "By Capability",
        items: [
          { key: "skills_on" as const, label: "Skills On", count: build_count("skills_on"), dot_class_name: "bg-violet-300" },
          { key: "skills_off" as const, label: "Skills Off", count: build_count("skills_off"), dot_class_name: "bg-amber-300" },
        ],
      },
    ];
  }, [agents, conversations_by_agent]);

  const header_trailing = (
    <>
      <WorkspaceSearchInput
        class_name="hidden xl:inline-flex"
        input_class_name="w-[220px]"
        on_change={set_search_query}
        placeholder="搜索成员、模型或路径"
        value={search_query}
      />

      <div className="hidden items-center gap-2 lg:flex">
        <WorkspaceIconToggleButton
          icon={<Grid2X2 className="h-4 w-4" />}
          is_active={view_mode === "grid"}
          onClick={() => set_view_mode("grid")}
        />
        <WorkspaceIconToggleButton
          icon={<List className="h-4 w-4" />}
          is_active={view_mode === "list"}
          onClick={() => set_view_mode("list")}
        />
      </div>

      <WorkspacePillButton onClick={on_create_agent}>
        新建成员
      </WorkspacePillButton>
    </>
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 gap-2 lg:gap-2.5 xl:gap-3">
      <ContactsFilterSidebar
        active_filter={active_filter}
        on_change_filter={set_active_filter}
        on_create_agent={on_create_agent}
        sections={filter_sections}
        total_count={agents.length}
      />

      <WorkspaceCanvasShell is_joined_with_inspector>
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <WorkspaceSurfaceHeader
            badge="CONTACTS"
            leading={<Users className="h-4 w-4 text-slate-800/72" />}
            subtitle={(
              <span className="truncate">
                {filtered_agents.length} / {agents.length} 个成员可见 · 浏览、筛选并发起 1v1 协作
              </span>
            )}
            title="成员目录"
            trailing={header_trailing}
          />

          <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
            {filtered_agents.length ? (
              <div className={
                view_mode === "grid"
                  ? "grid gap-5 xl:grid-cols-2 2xl:grid-cols-3"
                  : "space-y-4"
              }>
                {filtered_agents.map((agent) => {
                  const runtime_status = get_contacts_runtime_status(agent);
                  const status_label = get_contacts_runtime_label(runtime_status);
                  const is_selected = selected_agent?.agent_id === agent.agent_id;

                  return (
                    <ContactsAgentCard
                      key={agent.agent_id}
                      description={get_contacts_agent_description(agent)}
                      is_selected={is_selected}
                      model_label={get_contacts_model_label(agent)}
                      name={agent.name}
                      on_open_profile={() => navigate(AppRouteBuilders.contact_profile(agent.agent_id))}
                      on_open_room={() => on_open_direct_room(agent.agent_id)}
                      status_label={status_label}
                      status_tone={get_contacts_runtime_tone(runtime_status)}
                    />
                  );
                })}
              </div>
            ) : (
              <div className="workspace-card flex min-h-[420px] items-center justify-center rounded-[28px] px-8 text-center">
                <div>
                  <p className="text-[22px] font-bold tracking-[-0.04em] text-slate-950/90">没有符合条件的成员</p>
                  <p className="mt-3 text-sm leading-7 text-slate-700/60">
                    换一个筛选条件，或者直接创建一个新的成员继续配置。
                  </p>
                  <WorkspacePillButton class_name="mt-6" onClick={on_create_agent}>
                    新建成员
                  </WorkspacePillButton>
                </div>
              </div>
            )}
          </div>
        </div>
      </WorkspaceCanvasShell>

      <ContactsProfilePanel
        agent={selected_agent}
        conversations={selected_agent_conversations}
        on_delete_agent={on_delete_agent}
        on_edit_agent={on_edit_agent}
        on_open_room={on_open_direct_room}
        status_label={selected_agent
          ? get_contacts_runtime_label(get_contacts_runtime_status(selected_agent))
          : "未选择"}
        status_tone={selected_agent
          ? get_contacts_runtime_tone(get_contacts_runtime_status(selected_agent))
          : "default"}
      />
    </div>
  );
}
