"use client";

import { useCallback, useMemo, useState } from "react";
import {
  Check,
  CircleAlert,
  LoaderCircle,
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
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { CONVERSATION_FOCUS_MEDIA_QUERY } from "@/lib/layout/home-layout";
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

import {
  AgentCommunicationView,
  type AgentCommunicationViewState,
} from "./agent-communication-view";
import { ContactsAgentDetailActionsMenu } from "./contacts-agent-detail-actions-menu";

interface ContactsAgentDetailProps {
  agent: Agent;
  agents: Agent[];
  communication: AgentCommunicationViewState;
  onAddContact: (contactAgentId: string, alias: string) => Promise<boolean>;
  onBackToCommunicationDirectory: () => void;
  onCreateCommunicationConversation: (title?: string) => Promise<string | null>;
  onLoadOlderCommunicationMessages: () => Promise<boolean>;
  onCreateTeam: (agentId: string) => void;
  onDeleteAgent: (agentId: string) => void;
  onOpenDirectRoom: (agentId: string) => void;
  onRefreshCommunication: () => void;
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
  onBackToCommunicationDirectory,
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
  const isCompactLayout = useMediaQuery(CONVERSATION_FOCUS_MEDIA_QUERY);
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

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        activeTab={activeTab}
        compactTabsLabel={t("contacts.title")}
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

function AgentOptionsPersistenceStatus({
  state,
}: {
  state: AgentOptionsPersistenceState;
}) {
  const StatusIcon = state.phase === "saving"
    ? LoaderCircle
    : state.phase === "success"
      ? Check
      : state.phase === "error"
        ? CircleAlert
        : null;
  return (
    <span
      aria-live="polite"
      className={cn(
        "mr-1 inline-flex h-8 shrink-0 items-center gap-1 text-xs text-(--text-soft)",
        state.phase === "success" && "text-(--success)",
        state.phase === "error" && "text-(--destructive)",
      )}
      title={state.message}
    >
      {StatusIcon ? (
        <StatusIcon
          className={cn(
            "h-3.5 w-3.5",
            state.phase === "saving" && "animate-spin",
          )}
        />
      ) : null}
      <span className="max-sm:hidden">{state.message}</span>
    </span>
  );
}
