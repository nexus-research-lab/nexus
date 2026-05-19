"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ArrowLeft,
  Album,
  Bot,
  MessageSquareText,
  ToolCase,
  UserPen,
  Users,
} from "lucide-react";

import { AgentOptionsEditor } from "@/features/agents/options/agent-options-editor";
import type { TabKey } from "@/features/agents/options/components/agent-options-nav";
import { get_icon_avatar_src } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";

interface ContactsAgentDetailProps {
  agent: Agent;
  on_back: () => void;
  on_create_team: (agent_id: string) => void;
  on_delete_agent: (agent_id: string) => void;
  on_open_direct_room: (agent_id: string) => void;
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

/** 侧边栏联系人进入的内嵌 Agent 页面。 */
export function ContactsAgentDetail({
  agent,
  on_back,
  on_create_team,
  on_delete_agent,
  on_open_direct_room,
  on_save_agent_options,
  on_validate_agent_name,
}: ContactsAgentDetailProps) {
  const { t } = useI18n();
  const avatar_src = get_icon_avatar_src(agent.avatar);
  const [active_tab, set_active_tab] = useState<TabKey>("identity");

  const config_tabs = useMemo(
    () => [
      { key: "identity" as TabKey, label: t("agent_options.nav.identity"), icon: UserPen },
      { key: "advanced" as TabKey, label: t("agent_options.nav.tools"), icon: ToolCase },
      { key: "skills" as TabKey, label: t("agent_options.nav.skills"), icon: Album },
    ],
    [t],
  );

  const tag_labels = useMemo(() => {
    return (agent.vibe_tags ?? [])
      .map((tag) => tag.trim())
      .filter(Boolean);
  }, [agent.vibe_tags]);

  useEffect(() => {
    set_active_tab("identity");
  }, [agent.agent_id]);

  const initial_options = useMemo(
    () => ({
      provider: agent.options.provider,
      permission_mode: agent.options.permission_mode,
      allowed_tools: agent.options.allowed_tools,
      disallowed_tools: agent.options.disallowed_tools,
      max_turns: agent.options.max_turns,
      max_thinking_tokens: agent.options.max_thinking_tokens,
      setting_sources: agent.options.setting_sources,
    }),
    [
      agent.options.allowed_tools,
      agent.options.disallowed_tools,
      agent.options.max_thinking_tokens,
      agent.options.max_turns,
      agent.options.permission_mode,
      agent.options.provider,
      agent.options.setting_sources,
    ],
  );

  const handle_save = useCallback(
    async (
      title: string,
      options: AgentOptions,
      identity: AgentIdentityDraft,
    ) => {
      await on_save_agent_options(agent.agent_id, title, options, identity);
    },
    [agent.agent_id, on_save_agent_options],
  );

  const handle_validate_name = useCallback(
    async (name: string) => on_validate_agent_name(name, agent.agent_id),
    [agent.agent_id, on_validate_agent_name],
  );

  const trailing = (
    <div className="flex flex-wrap items-center justify-end gap-2">
      <WorkspaceSurfaceToolbarAction onClick={on_back}>
        <ArrowLeft className="h-3.5 w-3.5" />
        {t("contacts.back_to_agents")}
      </WorkspaceSurfaceToolbarAction>
      <WorkspaceSurfaceToolbarAction
        onClick={() => on_open_direct_room(agent.agent_id)}
        tone="primary"
      >
        <MessageSquareText className="h-3.5 w-3.5" />
        {t("contacts.chat")}
      </WorkspaceSurfaceToolbarAction>
      <WorkspaceSurfaceToolbarAction
        onClick={() => on_create_team(agent.agent_id)}
      >
        <Users className="h-3.5 w-3.5" />
        {t("contacts.create_team")}
      </WorkspaceSurfaceToolbarAction>
    </div>
  );

  const title_trailing = tag_labels.length ? (
    <div className="flex min-w-0 flex-wrap items-center gap-1.5">
      {tag_labels.map((tag) => (
        <span
          className="max-w-[120px] truncate rounded-full border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_68%,transparent)] px-2 py-0.5 text-[10.5px] font-semibold text-(--text-muted)"
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
        active_tab={active_tab}
        density="compact"
        leading={avatar_src ? (
          <img
            alt={agent.name}
            className="h-full w-full rounded-full object-cover"
            src={avatar_src}
          />
        ) : (
          <Bot className="h-4 w-4 text-(--icon-default)" />
        )}
        on_change_tab={set_active_tab}
        tabs={config_tabs}
        title={agent.name}
        title_trailing={title_trailing}
        trailing={trailing}
      />

      <AgentOptionsEditor
        active_tab={active_tab}
        agent_id={agent.agent_id}
        content_max_width_class_name="max-w-[860px]"
        hide_inline_nav
        initial_avatar={agent.avatar ?? ""}
        initial_description={agent.description ?? ""}
        initial_options={initial_options}
        initial_title={agent.name}
        initial_vibe_tags={agent.vibe_tags ?? []}
        is_active
        mode="edit"
        on_delete={on_delete_agent}
        on_save={handle_save}
        on_tab_change={set_active_tab}
        on_validate_name={handle_validate_name}
        show_cancel_button={false}
        show_delete_button
        variant="inline"
      />
    </div>
  );
}
