"use client";

import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";

import { get_default_agent_id, is_main_agent } from "@/config/options";
import { LauncherConsole } from "@/features/launcher/launcher-console";
import { get_launcher_surface_theme_style } from "@/features/launcher/launcher-surface-theme";
import { useLauncherPageController } from "@/hooks/launcher/use-launcher-page-controller";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { AgentOptions } from "@/shared/ui/dialog/agent-options";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { useTheme } from "@/shared/theme/theme-context";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { useAgentStore } from "@/store/agent";
import { SIDEBAR_SYSTEM_ITEM_IDS, useSidebarStore } from "@/store/sidebar";
import {
  AgentIdentityDraft,
  AgentOptions as AgentConfigOptions,
} from "@/types/agent/agent";

export function LauncherPage() {
  const { theme } = useTheme();
  const controller = useLauncherPageController();
  const navigate = useNavigate();
  const set_active_panel_item = useSidebarStore(
    (state) => state.set_active_panel_item,
  );
  const default_agent_id = get_default_agent_id();
  const [pending_delete_agent, set_pending_delete_agent] = useState<{
    id: string;
    name: string;
  } | null>(null);

  const open_agent_dm = useCallback(
    (agent_id: string, initial_prompt?: string) => {
      const next_active_item_id = is_main_agent(agent_id)
        ? SIDEBAR_SYSTEM_ITEM_IDS.nexus
        : agent_id;
      set_active_panel_item(next_active_item_id);

      void resolve_direct_room_navigation_target(agent_id, initial_prompt)
        .then(({ context, route }) => {
          controller.handle_select_agent(agent_id);
          set_active_panel_item(
            is_main_agent(agent_id)
              ? SIDEBAR_SYSTEM_ITEM_IDS.nexus
              : context.room.id,
          );
          navigate(route);
        })
        .catch((error) => {
          console.error("[LauncherPage] 打开 Agent DM 失败:", error);
        });
    },
    [controller, navigate, set_active_panel_item],
  );

  const handle_open_main_agent_dm = useCallback(
    (initial_prompt?: string) => {
      if (!default_agent_id) {
        console.error("[LauncherPage] 主智能体 ID 未就绪，无法打开 DM。");
        return;
      }
      open_agent_dm(default_agent_id, initial_prompt);
    },
    [default_agent_id, open_agent_dm],
  );

  const handle_select_agent = useCallback(
    (agent_id: string) => {
      open_agent_dm(agent_id);
    },
    [open_agent_dm],
  );

  const handle_save_agent_options = useCallback(
    async (
      _title: string,
      options: AgentConfigOptions,
      identity: AgentIdentityDraft,
    ) => {
      const should_open_room_after_create = controller.dialog_mode === "create";
      await controller.handle_save_agent_options(_title, options, identity);

      if (!should_open_room_after_create) {
        return;
      }

      const next_agent_id = useAgentStore.getState().current_agent_id;
      if (!next_agent_id) {
        return;
      }

      const { context, route } =
        await resolve_direct_room_navigation_target(next_agent_id);
      set_active_panel_item(context.room.id);
      navigate(route);
    },
    [controller, navigate, set_active_panel_item],
  );

  const handle_request_delete_agent = useCallback(
    (agent_id: string) => {
      const target_agent = controller.agents.find(
        (agent) => agent.id === agent_id,
      );
      controller.set_is_dialog_open(false);
      set_pending_delete_agent({
        id: agent_id,
        name: target_agent?.name ?? "该 Agent",
      });
    },
    [controller],
  );

  const handle_confirm_delete_agent = useCallback(async () => {
    if (!pending_delete_agent) {
      return;
    }

    await controller.handle_delete_agent(pending_delete_agent.id);
    set_pending_delete_agent(null);
  }, [controller, pending_delete_agent]);

  if (!controller.is_hydrated) {
    return <AppLoadingScreen />;
  }

  return (
    <>
      <div
        className="relative flex min-h-0 flex-1 overflow-hidden"
        style={get_launcher_surface_theme_style(theme)}
      >
        <LauncherConsole
          agents={controller.agents}
          rooms={controller.rooms}
          conversations={controller.conversations}
          current_agent_id={controller.current_agent_id}
          on_open_main_agent_dm={handle_open_main_agent_dm}
          on_select_agent={handle_select_agent}
        />
      </div>

      <AgentOptions
        agent_id={controller.editing_agent_id ?? undefined}
        mode={controller.dialog_mode}
        is_open={controller.is_dialog_open}
        on_close={() => {
          controller.set_is_dialog_open(false);
        }}
        on_delete={handle_request_delete_agent}
        on_save={handle_save_agent_options}
        on_validate_name={controller.handle_validate_agent_name}
        initial_avatar={controller.dialog_initial_avatar}
        initial_description={controller.dialog_initial_description}
        initial_title={controller.dialog_initial_title}
        initial_options={controller.dialog_initial_options}
        initial_vibe_tags={controller.dialog_initial_vibe_tags}
      />

      <ConfirmDialog
        confirm_text="删除成员"
        is_open={Boolean(pending_delete_agent)}
        message={`删除「${pending_delete_agent?.name ?? "该 Agent"}」后，该成员将不再出现在当前前端列表中。已有历史协作不会自动删除。`}
        on_cancel={() => set_pending_delete_agent(null)}
        on_confirm={() => {
          void handle_confirm_delete_agent();
        }}
        title="删除成员"
        variant="danger"
      />
    </>
  );
}
