/**
 * =====================================================
 * @File   : room-agent-about-surface.tsx
 * @Date   : 2026-04-15 15:08
 * @Author : leemysw
 * 2026-04-15 15:08   Create
 * =====================================================
 */

"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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
  roomId: string | null;
  conversationId: string | null;
  roomMembers: Agent[];
  isVisible: boolean;
  requestedAgentId?: string | null;
  requestedTab?: RoomAgentPanelTabKey;
  requestKey?: number;
  onSaveAgentOptions: (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => Promise<void>;
  onValidateAgentName: (
    name: string,
    agentId?: string,
  ) => Promise<AgentNameValidationResult>;
}

export function RoomAgentAboutSurface({
  agent,
  roomId: roomId,
  conversationId: conversationId,
  roomMembers: roomMembers,
  isVisible: isVisible,
  requestedAgentId: requestedAgentId,
  requestedTab: requestedTab,
  requestKey: requestKey,
  onSaveAgentOptions: onSaveAgentOptions,
  onValidateAgentName: onValidateAgentName,
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
      selectedId={selectedAgent.agent_id}
      onSelect={setSelectedAgentId}
    />
  ) : null;

  return (
    <WorkspaceSurfaceView
      bodyClassName="flex min-h-0 flex-1 flex-col px-0 py-0"
      bodyScrollable={false}
      contentClassName="flex h-full min-h-0 flex-1 flex-col"
      eyebrow={t("room.about")}
      maxWidthClassName="max-w-none"
      showEyebrow={false}
      showTitle={false}
      title={t("room.about")}
      titleTrailing={titleTrailing}
    >
      <div className="flex h-full min-h-0 flex-1 flex-col">
        <RoomAgentPanelTabs
          activeTab={activeTab}
          onChange={setActiveTab}
        />
        {activeTab === "private_domain" ? (
          <AgentPrivateDomainView
            agent={selectedAgent}
            conversationId={conversationId}
            roomId={roomId}
            variant="preview"
          />
        ) : (
          <AgentOptionsEditor
            activeTab={activeTab}
            agentId={selectedAgent.agent_id}
            contentMaxWidthClassName="max-w-[860px]"
            hideInlineNav
            initialAvatar={selectedAgent.avatar ?? ""}
            initialDescription={selectedAgent.description ?? ""}
            initialOptions={initialOptions}
            initialTitle={selectedAgent.name}
            initialVibeTags={selectedAgent.vibe_tags ?? []}
            isActive={isVisible}
            mode="edit"
            onSave={handleSave}
            onTabChange={setActiveTab}
            onValidateName={handleValidateName}
            showCancelButton={false}
            showDeleteButton={false}
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
  activeTab: activeTab,
  onChange: onChange,
}: {
  activeTab: RoomAgentPanelTabKey;
  onChange: (tab: RoomAgentPanelTabKey) => void;
}) {
  return (
    <div className="flex h-[41px] min-w-0 items-center border-b dialog-divider px-6">
      <UiUnderlineTabs
        activeValue={activeTab}
        ariaLabel="Agent 面板切换"
        className="-mx-0.5 flex-1 px-0.5"
        itemClassName="h-full"
        onChange={onChange}
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
