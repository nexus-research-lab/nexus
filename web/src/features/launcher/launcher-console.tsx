"use client";

import { useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AppRouteBuilders } from "@/app/router/route-paths";

import { ANIMATIONS } from "@/config/animation-assets";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { cn } from "@/lib/utils";
import { useSidebarStore } from "@/store/sidebar";
import { ConversationWithOwner } from "@/types/app/launcher";
import { query_launcher } from "@/lib/api/launcher-api";
import { ensure_direct_room, get_room_contexts } from "@/lib/api/room-api";
import { build_decorative_tokens, build_launcher_mention_targets, build_recent_launcher_entries } from "./launcher-console-helpers";
import { LauncherConsoleProps, RecentLauncherEntry } from "./launcher-console-types";
import { LauncherHeroStage } from "./launcher-hero-stage";

export function LauncherConsole({
  agents,
  conversations,
  rooms,
  current_agent_id,
  on_open_main_agent_dm,
  on_select_agent,
}: LauncherConsoleProps) {
  const [query, setQuery] = useState("");
  const [isQueryLoading, setIsQueryLoading] = useState(false);
  const navigate = useNavigate();
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);

  const agents_by_id = useMemo(
    () => new Map(agents.map((agent) => [agent.agent_id, agent])),
    [agents],
  );

  const conversations_with_owners = useMemo(() => {
    return conversations
      .map((conversation) => ({
        conversation,
        owner: conversation.agent_id ? agents_by_id.get(conversation.agent_id) ?? null : null,
      }))
      .sort((left, right) => right.conversation.last_activity_at - left.conversation.last_activity_at);
  }, [agents_by_id, conversations]);

  const decorative_tokens = useMemo(
    () => build_decorative_tokens(agents, conversations_with_owners),
    [agents, conversations_with_owners],
  );

  const mention_targets = useMemo(
    () => build_launcher_mention_targets(agents, rooms, conversations_with_owners),
    [agents, rooms, conversations_with_owners],
  );

  const recent_entries = useMemo(
    () => build_recent_launcher_entries(conversations_with_owners),
    [conversations_with_owners],
  );

  const handle_open_recent_entry = useCallback((entry: RecentLauncherEntry) => {
    void (async () => {
      try {
        if (entry.type === "dm" && entry.agent_id) {
          on_select_agent(entry.agent_id);
          const context = await ensure_direct_room(entry.agent_id);
          set_active_panel_item(context.room.id);
          navigate(
            AppRouteBuilders.room_conversation(
              context.room.id,
              context.conversation.id,
            ),
          );
          return;
        }

        if (!entry.room_id) {
          return;
        }

        if (entry.conversation_id) {
          set_active_panel_item(entry.room_id);
          navigate(
            AppRouteBuilders.room_conversation(entry.room_id, entry.conversation_id),
          );
          return;
        }

        const contexts = await get_room_contexts(entry.room_id);
        if (contexts.length > 0) {
          set_active_panel_item(entry.room_id);
          navigate(AppRouteBuilders.room_conversation(entry.room_id, contexts[0].conversation.id));
        }
      } catch (error) {
        console.error("Failed to open recent entry:", error);
      }
    })();
  }, [navigate, on_select_agent, set_active_panel_item]);

  const handle_submit = useCallback(async (next_query?: string) => {
    const trimmed = (next_query ?? query).trim();
    if (!trimmed || isQueryLoading) {
      return;
    }

    setIsQueryLoading(true);
    try {
      const action = await query_launcher({ query: trimmed });

      switch (action.action_type) {
        case "open_agent_dm": {
          const context = await ensure_direct_room(action.target_id);
          if (context) {
            set_active_panel_item(context.room.id);
            const route = AppRouteBuilders.room_conversation(context.room.id, context.conversation.id);
            const final_route = action.initial_message
              ? `${route}?initial=${encodeURIComponent(action.initial_message)}`
              : route;
            navigate(final_route);
          }
          break;
        }
        case "open_room": {
          const contexts = await get_room_contexts(action.target_id);
          if (contexts.length > 0) {
            set_active_panel_item(action.target_id);
            const route = AppRouteBuilders.room_conversation(action.target_id, contexts[0].conversation.id);
            const final_route = action.initial_message
              ? `${route}?initial=${encodeURIComponent(action.initial_message)}`
              : route;
            navigate(final_route);
          }
          break;
        }
      }
    } catch (error) {
      console.error("Launcher query failed:", error);
    } finally {
      setIsQueryLoading(false);
    }
  }, [query, isQueryLoading, navigate, set_active_panel_item]);

  const handle_enter_home = useCallback(() => {
    navigate(AppRouteBuilders.home());
  }, [navigate]);

  const handle_input_change = useCallback((value: string) => {
    setQuery(value);
  }, []);

  const handle_primary_action = useCallback((submitted_input: string) => {
    const trimmed_query = submitted_input.trim();
    if (!trimmed_query || isQueryLoading) {
      return false;
    }

    setQuery("");
    void handle_submit(trimmed_query);
    return true;
  }, [handle_submit, isQueryLoading]);

  return (
    <section className="relative flex h-full min-h-0 flex-1 flex-col overflow-hidden">
      <div className="pointer-events-none absolute left-3 top-3 z-20 sm:left-5 sm:top-4">
        <div className="relative flex items-center gap-1 px-1 py-1">
          <LottiePlayer
            class_name="pointer-events-none absolute left-10 -top-4 h-12 w-12 opacity-[0.72] sm:left-3 sm:-top-15 sm:h-30 sm:w-30"
            inline_style={undefined}
            src={ANIMATIONS.BOM}
          />
          <img alt="" className="h-9 w-9 sm:h-10 sm:w-10" src="/logo.webp" />
          <span className="text-[32px] font-semibold text-foreground mb-3"
            style={{
              fontFamily: "\"striper\", var(--font-sans)",
              fontWeight: 400,
            }}
          >
            nexus
          </span>
        </div>
      </div>
      <div className={cn(
        "relative flex min-h-0 flex-1 items-center justify-center px-8",
        "pb-8 pt-6",
      )}>
        <LauncherHeroStage
          current_agent_id={current_agent_id}
          decorative_tokens={decorative_tokens}
          mention_targets={mention_targets}
          on_enter_home={handle_enter_home}
          on_open_main_agent_dm={on_open_main_agent_dm}
          on_query_change={handle_input_change}
          on_select_agent={on_select_agent}
          on_open_recent_entry={handle_open_recent_entry}
          on_submit={handle_primary_action}
          query={query}
          recent_entries={recent_entries}
          is_query_loading={isQueryLoading}
        />
      </div>
    </section>
  );
}
