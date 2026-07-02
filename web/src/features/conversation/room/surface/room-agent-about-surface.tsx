/**
 * =====================================================
 * @File   : room-agent-about-surface.tsx
 * @Date   : 2026-04-15 15:08
 * @Author : leemysw
 * 2026-04-15 15:08   Create
 * =====================================================
 */

"use client";

import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";
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
import { UiUnderlineTabs } from "@/shared/ui/tabs";
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
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => Promise<void>;
  on_validate_agent_name: (
    name: string,
    agentId?: string,
  ) => Promise<AgentNameValidationResult>;
}

export function RoomAgentAboutSurface({
  agent,
  room_id: roomId,
  conversation_id: conversationId,
  room_members: roomMembers,
  header_action: headerAction,
  is_visible: isVisible,
  requested_agent_id: requestedAgentId,
  requested_tab: requestedTab,
  request_key: requestKey,
  on_save_agent_options: onSaveAgentOptions,
  on_validate_agent_name: onValidateAgentName,
}: RoomAgentAboutSurfaceProps) {
  const { t } = useI18n();
  const [selectedAgentId, setSelectedAgentId] = useState(agent.agent_id);
  const [activeTab, setActiveTab] = useState<RoomAgentPanelTabKey>("private_domain");

  useEffect(() => {
    setSelectedAgentId(requestedAgentId ?? agent.agent_id);
    setActiveTab(requestedTab ?? "private_domain");
  }, [agent.agent_id, requestKey, requestedAgentId, requestedTab]);

  const selectedAgent = useMemo(() => {
    return roomMembers.find((member) => member.agent_id === selectedAgentId) ?? agent;
  }, [agent, roomMembers, selectedAgentId]);

  const initialOptions = useMemo(() => ({
    provider: selectedAgent.options.provider,
    model: selectedAgent.options.model,
    permission_mode: selectedAgent.options.permission_mode,
    allowed_tools: selectedAgent.options.allowed_tools,
    disallowed_tools: selectedAgent.options.disallowed_tools,
    max_turns: selectedAgent.options.max_turns,
    max_thinking_tokens: selectedAgent.options.max_thinking_tokens,
    mcp_servers: selectedAgent.options.mcp_servers,
    setting_sources: selectedAgent.options.setting_sources,
  }), [
    selectedAgent.options.allowed_tools,
    selectedAgent.options.disallowed_tools,
    selectedAgent.options.max_thinking_tokens,
    selectedAgent.options.max_turns,
    selectedAgent.options.mcp_servers,
    selectedAgent.options.model,
    selectedAgent.options.permission_mode,
    selectedAgent.options.provider,
    selectedAgent.options.setting_sources,
  ]);

  const handleSave = useCallback(async (
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    await onSaveAgentOptions(selectedAgent.agent_id, title, options, identity);
  }, [onSaveAgentOptions, selectedAgent.agent_id]);

  const handleValidateName = useCallback(async (name: string) => {
    return onValidateAgentName(name, selectedAgent.agent_id);
  }, [onValidateAgentName, selectedAgent.agent_id]);

  const titleTrailing = roomMembers.length > 1 ? (
    <RoomAgentSwitcher
      members={roomMembers}
      selected_id={selectedAgent.agent_id}
      on_select={setSelectedAgentId}
    />
  ) : null;

  return (
    <WorkspaceSurfaceView
      action={headerAction}
      body_class_name="flex min-h-0 flex-1 flex-col px-0 py-0"
      body_scrollable={false}
      content_class_name="flex h-full min-h-0 flex-1 flex-col"
      eyebrow={t("room.about")}
      max_width_class_name="max-w-none"
      show_eyebrow={false}
      title={t("room.about")}
      title_trailing={titleTrailing}
    >
      <div className="flex h-full min-h-0 flex-1 flex-col">
        <RoomAgentPanelTabs
          active_tab={activeTab}
          on_change={setActiveTab}
        />
        {activeTab === "private_domain" ? (
          <AgentPrivateDomainView
            agent={selectedAgent}
            conversation_id={conversationId}
            room_id={roomId}
            variant="preview"
          />
        ) : (
          <AgentOptionsEditor
            active_tab={activeTab}
            agent_id={selectedAgent.agent_id}
            content_max_width_class_name="max-w-[860px]"
            hide_inline_nav
            initial_avatar={selectedAgent.avatar ?? ""}
            initial_description={selectedAgent.description ?? ""}
            initial_options={initialOptions}
            initial_title={selectedAgent.name}
            initial_vibe_tags={selectedAgent.vibe_tags ?? []}
            is_active={isVisible}
            mode="edit"
            on_save={handleSave}
            on_tab_change={setActiveTab}
            on_validate_name={handleValidateName}
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
  active_tab: activeTab,
  on_change: onChange,
}: {
  active_tab: RoomAgentPanelTabKey;
  on_change: (tab: RoomAgentPanelTabKey) => void;
}) {
  return (
    <div className="flex h-[41px] min-w-0 items-center border-b dialog-divider px-6">
      <UiUnderlineTabs
        active_value={activeTab}
        aria_label="Agent 面板切换"
        class_name="-mx-0.5 flex-1 px-0.5"
        item_class_name="h-full"
        on_change={onChange}
        options={ROOM_AGENT_PANEL_TABS.map((item) => ({
          icon: item.icon,
          label: item.label,
          title: item.label,
          value: item.key,
        }))}
      />
    </div>
  );
}
