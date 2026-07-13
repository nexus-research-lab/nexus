"use client";

import { useCallback, useMemo } from "react";
import {
  ArrowLeft,
  Album,
  Brain,
  Handshake,
  MessageSquareText,
  ToolCase,
  UserPen,
  Users,
} from "lucide-react";

import { AgentPrivateDomainView } from "@/features/agents/private-domain/agent-private-domain-view";
import { AgentOptionsInlineEditor } from "@/features/agents/options/agent-options-editor";
import {
  buildAgentOptionsEditSource,
  type AgentOptionsTabKey,
} from "@/features/agents/options/agent-options-editor-model";
import { AgentMemoryView } from "@/features/memory/agent-memory-view";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";

interface ContactsAgentDetailProps {
  agent: Agent;
  onBack: () => void;
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

type ContactDetailTabKey = AgentOptionsTabKey | "private_domain" | "memory";

/** 侧边栏联系人进入的内嵌 Agent 页面。 */
export function ContactsAgentDetail({
  agent,
  onBack,
  onCreateTeam,
  onDeleteAgent,
  onOpenDirectRoom,
  onSaveAgentOptions,
  onValidateAgentName,
}: ContactsAgentDetailProps) {
  const { t } = useI18n();
  const [activeTab, setActiveTab] = useResettableState<ContactDetailTabKey>(
    "private_domain",
    agent.agent_id,
  );

  const configTabs = useMemo(
    () => [
      { key: "private_domain" as ContactDetailTabKey, label: "联络", icon: Handshake },
      { key: "memory" as ContactDetailTabKey, label: t("capability.memory"), icon: Brain },
      { key: "identity" as AgentOptionsTabKey, label: t("agent_options.nav.identity"), icon: UserPen },
      { key: "advanced" as AgentOptionsTabKey, label: t("agent_options.nav.tools"), icon: ToolCase },
      { key: "skills" as AgentOptionsTabKey, label: t("agent_options.nav.skills"), icon: Album },
    ],
    [t],
  );

  const tagLabels = useMemo(() => {
    return (agent.vibe_tags ?? [])
      .map((tag) => tag.trim())
      .filter(Boolean);
  }, [agent.vibe_tags]);

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

  const trailing = (
    <div className="flex flex-wrap items-center justify-end gap-2">
      <WorkspaceSurfaceToolbarAction onClick={onBack}>
        <ArrowLeft className="h-3.5 w-3.5" />
        {t("contacts.back_to_agents")}
      </WorkspaceSurfaceToolbarAction>
      <WorkspaceSurfaceToolbarAction
        onClick={() => onOpenDirectRoom(agent.agent_id)}
        tone="primary"
      >
        <MessageSquareText className="h-3.5 w-3.5" />
        {t("contacts.chat")}
      </WorkspaceSurfaceToolbarAction>
      <WorkspaceSurfaceToolbarAction
        onClick={() => onCreateTeam(agent.agent_id)}
      >
        <Users className="h-3.5 w-3.5" />
        {t("contacts.create_team")}
      </WorkspaceSurfaceToolbarAction>
    </div>
  );

  const titleTrailing = tagLabels.length ? (
    <div className="flex min-w-0 flex-wrap items-center gap-1.5">
      {tagLabels.map((tag) => (
        <span
          className="max-w-[120px] truncate rounded-[6px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-transparent px-2 py-0.5 text-[10.5px] font-medium text-(--text-muted)"
          key={tag}
          title={tag}
        >
          {tag}
        </span>
      ))}
    </div>
  ) : null;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        activeTab={activeTab}
        leading={<UiAgentAvatar avatar={agent.avatar} className="h-full w-full border-0 shadow-none" name={agent.name} size="sm" />}
        onChangeTab={setActiveTab}
        tabs={configTabs}
        title={agent.name}
        titleTrailing={titleTrailing}
        trailing={trailing}
      />

      {activeTab === "private_domain" ? (
        <AgentPrivateDomainView agent={agent} />
      ) : activeTab === "memory" ? (
        <AgentMemoryView agent={agent} />
      ) : (
        <AgentOptionsInlineEditor
          activeTab={activeTab}
          contentMaxWidthClassName={WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME}
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
