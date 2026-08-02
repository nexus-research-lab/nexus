"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";

import {
  AGENT_DETAIL_TABS,
  type AgentDetailTabKey,
} from "@/features/agents/agent-detail-navigation";
import { AgentPrivateDomainView } from "@/features/agents/private-domain/agent-private-domain-view";
import { AgentOptionsInlineEditor } from "@/features/agents/options/agent-options-editor";
import {
  buildAgentOptionsEditSource,
} from "@/features/agents/options/agent-options-editor-model";
import { AgentMemoryView } from "@/features/memory/agent-memory-view";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiTabs } from "@/shared/ui/navigation/tabs";
import {
  WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
  WORKSPACE_PANEL_HEADER_PADDING_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type { Agent, AgentIdentityDraft, AgentNameValidationResult, AgentOptions } from "@/types/agent/agent";

import { RoomAgentSwitcher } from "./room-agent-switcher";

interface RoomAgentAboutSurfaceProps {
  agent: Agent;
  roomId: string | null;
  conversationId: string | null;
  roomMembers: Agent[];
  isVisible: boolean;
  requestedAgentId?: string | null;
  requestedTab?: AgentDetailTabKey;
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
  roomId,
  conversationId,
  roomMembers,
  isVisible,
  requestedAgentId,
  requestedTab,
  requestKey,
  onSaveAgentOptions,
  onValidateAgentName,
}: RoomAgentAboutSurfaceProps) {
  const { t } = useI18n();
  const [selectedAgentId, setSelectedAgentId] = useState(agent.agent_id);
  const [activeTab, setActiveTab] = useState<AgentDetailTabKey>("identity");

  useEffect(() => {
    setSelectedAgentId(requestedAgentId ?? agent.agent_id);
    setActiveTab(requestedTab ?? "identity");
  }, [agent.agent_id, requestKey, requestedAgentId, requestedTab]);

  const selectedAgent = useMemo(() => {
    return roomMembers.find((member) => member.agent_id === selectedAgentId) ?? agent;
  }, [agent, roomMembers, selectedAgentId]);

  const editorSource = useMemo(
    () => buildAgentOptionsEditSource(selectedAgent),
    [selectedAgent],
  );

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

  const agentSwitcher = roomMembers.length > 1 ? (
    <RoomAgentSwitcher
      members={roomMembers}
      selectedId={selectedAgent.agent_id}
      onSelect={setSelectedAgentId}
      variant="panel"
    />
  ) : null;

  return (
    <WorkspaceSurfaceView
      bodyClassName="flex min-h-0 flex-1 flex-col px-0 py-0"
      bodyScrollable={false}
      contentClassName="flex h-full min-h-0 flex-1 flex-col"
      maxWidthClassName="max-w-none"
      title={t("room.about")}
    >
      <div className="flex h-full min-h-0 flex-1 flex-col">
        <RoomAgentPanelTabs
          activeTab={activeTab}
          leading={agentSwitcher}
          onChange={setActiveTab}
        />
        {activeTab === "private_domain" ? (
          <AgentPrivateDomainView
            agent={selectedAgent}
            conversationId={conversationId}
            roomId={roomId}
            variant="preview"
          />
        ) : activeTab === "memory" ? (
          <AgentMemoryView agent={selectedAgent} />
        ) : (
          <AgentOptionsInlineEditor
            activeTab={activeTab}
            contentMaxWidthClassName="max-w-[860px]"
            isActive={isVisible}
            onSave={handleSave}
            onTabChange={setActiveTab}
            onValidateName={handleValidateName}
            showDeleteButton={false}
            source={editorSource}
          />
        )}
      </div>
    </WorkspaceSurfaceView>
  );
}

function RoomAgentPanelTabs({
  activeTab: activeTab,
  leading: leading,
  onChange: onChange,
}: {
  activeTab: AgentDetailTabKey;
  leading?: ReactNode;
  onChange: (tab: AgentDetailTabKey) => void;
}) {
  const { t } = useI18n();

  return (
    <div className={cn(
      "flex min-w-0 shrink-0 items-center border-b dialog-divider",
      WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
      WORKSPACE_PANEL_HEADER_PADDING_CLASS,
    )}>
      {leading ? (
        <div className="mr-5 shrink-0">
          {leading}
        </div>
      ) : null}
      <UiTabs
        activeValue={activeTab}
        ariaLabel={t("room.agent_panel_tabs")}
        className="-mx-0.5 min-w-0 flex-1 px-0.5"
        density="compact"
        itemClassName="h-7 px-2.5"
        onChange={onChange}
        options={AGENT_DETAIL_TABS.map((tab) => ({
          label: t(tab.labelKey),
          title: t(tab.labelKey),
          value: tab.key,
        }))}
      />
    </div>
  );
}
