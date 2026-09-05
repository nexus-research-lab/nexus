/**
 * INPUT: 当前 Agent、可编辑配置、联络资源与目录/协作导航命令。
 * OUTPUT: 带显式桌面目录返回入口的 Agent 分栏详情页。
 * POS: 联系人目录的二级页面；手机返回由应用页头承载。
 */
"use client";

import { useCallback, useMemo, useState } from "react";
import {
  ArrowLeft,
  MessageCirclePlus,
  MessageSquareText,
  Trash2,
} from "lucide-react";

import {
  AGENT_DETAIL_TABS,
  type AgentDetailTabKey,
} from "@/features/agents/agent-detail-navigation";
import { AgentOptionsInlineEditor } from "@/features/agents/options/agent-options-editor";
import {
  buildAgentOptionsEditSource,
} from "@/features/agents/options/agent-options-editor-model";
import type {
  AgentOptionsPersistenceState,
  AgentOptionsTabKey,
} from "@/features/agents/options/agent-options-editor-model";
import { AgentMemoryView } from "@/features/memory/agent-memory-view";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { APP_NARROW_VIEWPORT_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  WORKSPACE_CONTENT_GUTTER_CLASS_NAME,
  WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME,
} from "@/shared/ui/layout/workspace-content-layout";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";
import type { AgentCommunicationReadFailureKind } from "@/types/agent/communication";

import {
  AgentCommunicationView,
  type AgentCommunicationViewState,
} from "./agent-communication-view";
import { AgentOptionsPersistenceStatus } from "./agent-options-persistence-status";
import { ContactsAgentDetailActionsMenu } from "./contacts-agent-detail-actions-menu";

interface ContactsAgentDetailProps {
  agent: Agent;
  agents: Agent[];
  communication: AgentCommunicationViewState;
  onAddContact: (contactAgentId: string, alias: string) => Promise<boolean>;
  onBackToAgentDirectory: () => void;
  onBackToCommunicationDirectory: () => void;
  onClearCommunicationMutationFailure: () => void;
  onCreateCommunicationConversation: (title?: string) => Promise<string | null>;
  onLoadOlderCommunicationMessages: () => Promise<boolean>;
  onCreateTeam: (agentId: string) => void;
  onDeleteAgent: (agentId: string) => void;
  onOpenDirectRoom: (agentId: string) => void;
  onRefreshCommunication: (kind?: AgentCommunicationReadFailureKind) => void;
  onRemoveCommunicationContact: (contactAgentId: string) => Promise<boolean>;
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
  onSelectCommunicationConversation: (conversationId: string) => void;
  onSelectCommunicationContact: (contactAgentId: string) => void;
  onSendCommunicationMessage: (content: string) => Promise<void>;
}

/** 侧边栏联系人进入的内嵌 Agent 页面。 */
export function ContactsAgentDetail({
  agent,
  agents,
  communication,
  onAddContact,
  onBackToAgentDirectory,
  onBackToCommunicationDirectory,
  onClearCommunicationMutationFailure,
  onCreateCommunicationConversation,
  onLoadOlderCommunicationMessages,
  onCreateTeam,
  onDeleteAgent,
  onOpenDirectRoom,
  onRefreshCommunication,
  onRemoveCommunicationContact,
  onSaveAgentOptions,
  onValidateAgentName,
  onSelectCommunicationContact,
  onSelectCommunicationConversation,
  onSendCommunicationMessage,
}: ContactsAgentDetailProps) {
  const { t } = useI18n();
  const isCompactLayout = useMediaQuery(APP_NARROW_VIEWPORT_MEDIA_QUERY);
  const [activeTab, setActiveTab] = useState<AgentDetailTabKey>("identity");
  const [persistenceState, setPersistenceState] =
    useResettableState<AgentOptionsPersistenceState>({
      message: t("agent_options.auto_save"),
      phase: "idle",
    }, agent.agent_id);
  const isEditorTab = isAgentOptionsTab(activeTab);

  const configTabs = useMemo(
    () => AGENT_DETAIL_TABS.map((tab) => ({
      key: tab.key,
      label: t(tab.labelKey),
    })),
    [t],
  );

  const editorSource = useMemo(
    () => buildAgentOptionsEditSource(agent),
    [agent],
  );

  const handleSave = useCallback(
    async (
      title: string,
      options: AgentOptions,
      identity: AgentIdentityDraft,
    ) => {
      await onSaveAgentOptions(agent.agent_id, title, options, identity);
    },
    [agent.agent_id, onSaveAgentOptions],
  );

  const handleValidateName = useCallback(
    async (name: string) => onValidateAgentName(name, agent.agent_id),
    [agent.agent_id, onValidateAgentName],
  );

  const actionControls = isCompactLayout ? (
    <ContactsAgentDetailActionsMenu
      onCreateTeam={() => onCreateTeam(agent.agent_id)}
      onDelete={() => onDeleteAgent(agent.agent_id)}
      onOpenDirectRoom={() => onOpenDirectRoom(agent.agent_id)}
    />
  ) : (
    <>
      <UiButton
        onClick={() => onOpenDirectRoom(agent.agent_id)}
        size="sm"
        variant="ghost"
      >
        <MessageSquareText className="h-4 w-4" />
        {t("contacts.chat")}
      </UiButton>
      <UiButton
        onClick={() => onCreateTeam(agent.agent_id)}
        size="sm"
        variant="ghost"
      >
        <MessageCirclePlus className="h-4 w-4" />
        {t("contacts.create_team")}
      </UiButton>
      <UiIconButton
        aria-label={t("agent_options.delete_agent")}
        onClick={() => onDeleteAgent(agent.agent_id)}
        size="md"
        title={t("agent_options.delete_agent")}
        tone="danger"
        variant="ghost"
      >
        <Trash2 className="h-4 w-4" />
      </UiIconButton>
    </>
  );
  const trailing = (
    <div className="flex shrink-0 items-center justify-end gap-0.5">
      {isEditorTab ? (
        <AgentOptionsPersistenceStatus state={persistenceState} />
      ) : null}
      {actionControls}
    </div>
  );
  const directoryNavigation = !isCompactLayout ? (
    <UiButton
      aria-label={t("contacts.back_to_directory")}
      onClick={onBackToAgentDirectory}
      size="sm"
      title={t("contacts.back_to_directory")}
      variant="ghost"
    >
      <ArrowLeft className="h-4 w-4" />
      {t("contacts.title")}
    </UiButton>
  ) : undefined;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        activeTab={activeTab}
        compactTabsLabel={t("contacts.title")}
        leading={directoryNavigation}
        leadingClassName="!h-auto !w-auto !bg-transparent"
        onChangeTab={setActiveTab}
        tabs={configTabs}
        trailing={trailing}
      />
      <div
        aria-hidden="true"
        className="mx-[var(--workspace-content-gutter)] shrink-0 border-t border-(--divider-subtle-color)"
      />

      <div className={cn(
        "min-h-0 flex-1 flex-col",
        isEditorTab ? "flex" : "hidden",
      )}>
        <AgentOptionsInlineEditor
          activeTab={isEditorTab ? activeTab : "identity"}
          contentMaxWidthClassName={WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME}
          isActive
          onPersistenceStateChange={setPersistenceState}
          onSave={handleSave}
          onTabChange={setActiveTab}
          onValidateName={handleValidateName}
          saveMode="automatic"
          source={editorSource}
        />
      </div>

      {activeTab === "private_domain" || activeTab === "memory" ? (
        <div className={cn(
          WORKSPACE_CONTENT_GUTTER_CLASS_NAME,
          "flex min-h-0 min-w-0 flex-1",
        )}>
          {activeTab === "private_domain" ? (
            <AgentCommunicationView
              agent={agent}
              agents={agents}
              onAddContact={onAddContact}
              onBackToDirectory={onBackToCommunicationDirectory}
              onClearMutationFailure={onClearCommunicationMutationFailure}
              onCreateConversation={onCreateCommunicationConversation}
              onLoadOlderMessages={onLoadOlderCommunicationMessages}
              onRefresh={onRefreshCommunication}
              onRemoveContact={onRemoveCommunicationContact}
              onSelectContact={onSelectCommunicationContact}
              onSelectConversation={onSelectCommunicationConversation}
              onSendMessage={onSendCommunicationMessage}
              state={communication}
            />
          ) : (
            <AgentMemoryView agent={agent} />
          )}
        </div>
      ) : null}

    </div>
  );
}

function isAgentOptionsTab(tab: AgentDetailTabKey): tab is AgentOptionsTabKey {
  return tab === "identity" || tab === "skills" || tab === "advanced";
}
