/**
 * =====================================================
 * @File   : room-agent-about-surface.tsx
 * @Date   : 2026-04-15 15:08
 * @Author : leemysw
 * 2026-04-15 15:08   Create
 * =====================================================
 */

"use client";

import { ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import {
  Album,
  Handshake,
  ToolCase,
  UserPen,
  type LucideIcon,
} from "lucide-react";

import { AgentPrivateDomainView } from "@/features/agents/private-domain/agent-private-domain-view";
import { AgentOptionsEditor } from "@/features/agents/options/agent-options-editor";
import type { TabKey } from "@/features/agents/options/components/agent-options-nav";
import { cn } from "@/lib/utils";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import { AgentIdentityDraft, AgentNameValidationResult, AgentOptions, Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";
import { RoomAgentSwitcher } from "./room-agent-switcher";

type RoomAgentPanelTabKey = TabKey | "private_domain";

interface RoomAgentAboutSurfaceProps {
  agent: Agent;
  room_id: string | null;
  conversation_id: string | null;
  room_members: Agent[];
  header_action?: ReactNode;
  is_visible: boolean;
  requested_agent_id?: string | null;
  requested_tab?: RoomAgentPanelTabKey;
  request_key?: number;
  on_save_agent_options: (
    agent_id: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => Promise<void>;
  on_validate_agent_name: (
    name: string,
    agent_id?: string,
  ) => Promise<AgentNameValidationResult>;
}

export function RoomAgentAboutSurface({
  agent,
  room_id,
  conversation_id,
  room_members,
  header_action,
  is_visible,
  requested_agent_id,
  requested_tab,
  request_key,
  on_save_agent_options,
  on_validate_agent_name,
}: RoomAgentAboutSurfaceProps) {
  const { t } = useI18n();
  const [selected_agent_id, set_selected_agent_id] = useState(agent.agent_id);
  const [active_tab, set_active_tab] = useState<RoomAgentPanelTabKey>("private_domain");

  useEffect(() => {
    set_selected_agent_id(requested_agent_id ?? agent.agent_id);
    set_active_tab(requested_tab ?? "private_domain");
  }, [agent.agent_id, request_key, requested_agent_id, requested_tab]);

  const selected_agent = useMemo(() => {
    return room_members.find((member) => member.agent_id === selected_agent_id) ?? agent;
  }, [agent, room_members, selected_agent_id]);

  const initial_options = useMemo(() => ({
    provider: selected_agent.options.provider,
    permission_mode: selected_agent.options.permission_mode,
    allowed_tools: selected_agent.options.allowed_tools,
    disallowed_tools: selected_agent.options.disallowed_tools,
    setting_sources: selected_agent.options.setting_sources,
  }), [
    selected_agent.options.allowed_tools,
    selected_agent.options.disallowed_tools,
    selected_agent.options.permission_mode,
    selected_agent.options.provider,
    selected_agent.options.setting_sources,
  ]);

  const handle_save = useCallback(async (
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    await on_save_agent_options(selected_agent.agent_id, title, options, identity);
  }, [on_save_agent_options, selected_agent.agent_id]);

  const handle_validate_name = useCallback(async (name: string) => {
    return on_validate_agent_name(name, selected_agent.agent_id);
  }, [on_validate_agent_name, selected_agent.agent_id]);

  const title_trailing = room_members.length > 1 ? (
    <RoomAgentSwitcher
      members={room_members}
      selected_id={selected_agent.agent_id}
      on_select={set_selected_agent_id}
    />
  ) : null;

  return (
    <WorkspaceSurfaceView
      action={header_action}
      body_class_name="flex min-h-0 flex-1 flex-col px-0 py-0"
      eyebrow={t("room.about")}
      max_width_class_name="max-w-none"
      show_eyebrow={false}
      title={t("room.about")}
      title_trailing={title_trailing}
    >
      <div className="flex min-h-0 flex-1 flex-col">
        <RoomAgentPanelTabs
          active_tab={active_tab}
          on_change={set_active_tab}
        />
        {active_tab === "private_domain" ? (
          <AgentPrivateDomainView
            agent={selected_agent}
            conversation_id={conversation_id}
            room_id={room_id}
            variant="preview"
          />
        ) : (
          <AgentOptionsEditor
            active_tab={active_tab}
            agent_id={selected_agent.agent_id}
            content_max_width_class_name="max-w-[860px]"
            hide_inline_nav
            initial_avatar={selected_agent.avatar ?? ""}
            initial_description={selected_agent.description ?? ""}
            initial_options={initial_options}
            initial_title={selected_agent.name}
            initial_vibe_tags={selected_agent.vibe_tags ?? []}
            is_active={is_visible}
            mode="edit"
            on_save={handle_save}
            on_tab_change={set_active_tab}
            on_validate_name={handle_validate_name}
            show_cancel_button={false}
            show_delete_button={false}
            variant="inline"
          />
        )}
      </div>
    </WorkspaceSurfaceView>
  );
}

const ROOM_AGENT_PANEL_TABS: Array<{
  key: RoomAgentPanelTabKey;
  label: string;
  icon: LucideIcon;
}> = [
  { key: "private_domain", label: "联络", icon: Handshake },
  { key: "identity", label: "身份", icon: UserPen },
  { key: "advanced", label: "工具", icon: ToolCase },
  { key: "skills", label: "技能", icon: Album },
];

function RoomAgentPanelTabs({
  active_tab,
  on_change,
}: {
  active_tab: RoomAgentPanelTabKey;
  on_change: (tab: RoomAgentPanelTabKey) => void;
}) {
  return (
    <div className="flex h-[41px] min-w-0 items-center border-b dialog-divider px-6">
      <nav
        aria-label="Agent 面板切换"
        className="soft-scrollbar scrollbar-hide -mx-0.5 flex min-w-0 flex-1 items-center gap-4 overflow-x-auto px-0.5"
      >
        {ROOM_AGENT_PANEL_TABS.map((item) => {
          const Icon = item.icon;
          const is_active = active_tab === item.key;
          return (
            <button
              aria-current={is_active ? "page" : undefined}
              aria-pressed={is_active}
              className={cn(
                "inline-flex h-full shrink-0 items-center gap-1.5 border-b-2 border-transparent px-0 py-0 text-[11px] font-semibold transition-[color,border-color] duration-(--motion-duration-fast) ease-out",
                is_active
                  ? "border-(--surface-interactive-active-border) text-(--text-strong)"
                  : "text-(--text-default) hover:text-(--text-strong)",
              )}
              key={item.key}
              onClick={() => on_change(item.key)}
              title={item.label}
              type="button"
            >
              <Icon className="h-3.5 w-3.5" />
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>
    </div>
  );
}
