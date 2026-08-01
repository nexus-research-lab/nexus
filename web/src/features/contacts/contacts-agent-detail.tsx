"use client";

import { useCallback, useMemo } from "react";
import {
  MessageSquareText,
  Users,
} from "lucide-react";

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
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { CONVERSATION_FOCUS_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";

import { ContactsAgentDetailActionsMenu } from "./contacts-agent-detail-actions-menu";

interface ContactsAgentDetailProps {
  agent: Agent;
  onCreateTeam: (agentId: string) => void;
  onDeleteAgent: (agentId: string) => void;
  onOpenDirectRoom: (agentId: string) => void;
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

/** 侧边栏联系人进入的内嵌 Agent 页面。 */
export function ContactsAgentDetail({
  agent,
  onCreateTeam,
  onDeleteAgent,
  onOpenDirectRoom,
  onSaveAgentOptions,
  onValidateAgentName,
}: ContactsAgentDetailProps) {
  const { t } = useI18n();
  const isCompactLayout = useMediaQuery(CONVERSATION_FOCUS_MEDIA_QUERY);
  const [activeTab, setActiveTab] = useResettableState<AgentDetailTabKey>(
    "identity",
    agent.agent_id,
  );

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

  const trailing = isCompactLayout ? (
    <ContactsAgentDetailActionsMenu
      onCreateTeam={() => onCreateTeam(agent.agent_id)}
      onOpenDirectRoom={() => onOpenDirectRoom(agent.agent_id)}
    />
  ) : (
    <div className="flex shrink-0 items-center justify-end gap-0.5">
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
        <Users className="h-4 w-4" />
        {t("contacts.create_team")}
      </UiButton>
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

      {activeTab === "private_domain" ? (
        <AgentPrivateDomainView agent={agent} />
      ) : activeTab === "memory" ? (
        <AgentMemoryView agent={agent} />
      ) : (
        <AgentOptionsInlineEditor
          activeTab={activeTab}
          contentMaxWidthClassName={WORKSPACE_CONTENT_MAX_WIDTH_CLASS_NAME}
          isActive
          onDelete={onDeleteAgent}
          onSave={handleSave}
          onTabChange={setActiveTab}
          onValidateName={handleValidateName}
          showDeleteButton
          source={editorSource}
        />
      )}
    </div>
  );
}
