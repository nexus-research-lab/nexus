// INPUT: 当前 Room/Agent、成员目录、显式简介请求及既有配置保存命令。
// OUTPUT: 当前作用域内的成员/栏目导航、共享设置/记忆/联络工作面。
// POS: Room 简介装配；只拥有导航选择，字段、保存和内容状态由各领域负责。
"use client";

import { useCallback, useMemo, useSyncExternalStore, type ReactNode } from "react";

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
import { captureAuthOwnerScopeGeneration, subscribeAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
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
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration,
  );
  // 简介导航属于 Room/Agent；同一 Room 的 Session 切换由内容领域更新查询。
  const navigationScope = JSON.stringify([
    ownerGeneration, roomId, agent.agent_id, requestKey, requestedAgentId, requestedTab,
  ]);
  const [selectedAgentId, setSelectedAgentId] = useResettableState(
    requestedAgentId ?? agent.agent_id, navigationScope,
  );
  const [activeTab, setActiveTab] = useResettableState<AgentDetailTabKey>(
    requestedTab ?? "identity", navigationScope,
  );
  const selectedAgent = roomMembers.find((member) => member.agent_id === selectedAgentId) ?? agent;
  // 提交当前回退，成员重新出现时不恢复已经失效的旧选择。
  if (selectedAgentId !== selectedAgent.agent_id) {
    setSelectedAgentId(selectedAgent.agent_id);
  }
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
  activeTab,
  leading,
  onChange,
}: {
  activeTab: AgentDetailTabKey;
  leading?: ReactNode;
  onChange: (tab: AgentDetailTabKey) => void;
}) {
  const { t } = useI18n();

  return (
    <div className={cn(
      "flex min-w-0 shrink-0 items-center gap-2 border-b dialog-divider",
      WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
      WORKSPACE_PANEL_HEADER_PADDING_CLASS,
    )}>
      {leading ? (
        <div className="shrink-0">
          {leading}
        </div>
      ) : null}
      <UiTabs
        activeValue={activeTab}
        ariaLabel={t("room.agent_panel_tabs")}
        className="-mx-0.5 min-w-0 flex-1 px-0.5"
        density="compact"
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
