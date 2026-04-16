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

import { AgentOptionsEditor } from "@/features/agents/options/agent-options-editor";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import { AgentIdentityDraft, AgentNameValidationResult, AgentOptions, Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";
import { RoomAgentSwitcher } from "./room-agent-switcher";

interface RoomAgentAboutSurfaceProps {
  agent: Agent;
  room_members: Agent[];
  header_action?: ReactNode;
  is_visible: boolean;
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
  room_members,
  header_action,
  is_visible,
  on_save_agent_options,
  on_validate_agent_name,
}: RoomAgentAboutSurfaceProps) {
  const { t } = useI18n();
  const [selected_agent_id, set_selected_agent_id] = useState(agent.agent_id);

  useEffect(() => {
    set_selected_agent_id(agent.agent_id);
  }, [agent.agent_id]);

  const selected_agent = useMemo(() => {
    return room_members.find((member) => member.agent_id === selected_agent_id) ?? agent;
  }, [agent, room_members, selected_agent_id]);

  const initial_options = useMemo(() => ({
    provider: selected_agent.options.provider,
    permission_mode: selected_agent.options.permission_mode,
    allowed_tools: selected_agent.options.allowed_tools,
    disallowed_tools: selected_agent.options.disallowed_tools,
    system_prompt: selected_agent.options.system_prompt,
    setting_sources: selected_agent.options.setting_sources,
  }), [
    selected_agent.options.allowed_tools,
    selected_agent.options.disallowed_tools,
    selected_agent.options.permission_mode,
    selected_agent.options.provider,
    selected_agent.options.setting_sources,
    selected_agent.options.system_prompt,
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
        <AgentOptionsEditor
          agent_id={selected_agent.agent_id}
          mode="edit"
          is_active={is_visible}
          on_save={handle_save}
          on_validate_name={handle_validate_name}
          initial_title={selected_agent.name}
          initial_options={initial_options}
          initial_avatar={selected_agent.avatar ?? ""}
          initial_description={selected_agent.description ?? ""}
          initial_vibe_tags={selected_agent.vibe_tags ?? []}
          content_max_width_class_name="max-w-[860px]"
          show_cancel_button={false}
          show_delete_button={false}
          variant="inline"
        />
      </div>
    </WorkspaceSurfaceView>
  );
}
