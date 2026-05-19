"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AppRouteBuilders } from "@/app/router/route-paths";

import { ANIMATIONS } from "@/config/animation-assets";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { cn } from "@/lib/utils";
import { useSidebarStore } from "@/store/sidebar";
import { query_launcher } from "@/lib/api/launcher-api";
import { ensure_direct_room, get_room_contexts } from "@/lib/api/room-api";
import { Bot, Clock3, Compass, CornerDownLeft, Hash, Search } from "lucide-react";
import {
  build_launcher_tour,
} from "@/features/launcher/launcher-tour";
import { usePageOnboardingTour } from "@/shared/ui/onboarding/use-page-onboarding-tour";
import {
  build_decorative_tokens,
  build_launcher_mention_targets,
  build_recent_launcher_entries,
} from "./launcher-console-helpers";
import {
  LauncherConsoleProps,
  RecentLauncherEntry,
} from "./launcher-console-types";
import { LauncherHeroStage } from "./launcher-hero-stage";

export function LauncherConsole({
  agents,
  rooms,
  conversations,
  current_agent_id,
  on_open_main_agent_dm,
  on_open_route,
  on_select_agent,
  variant = "full",
}: LauncherConsoleProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [isQueryLoading, setIsQueryLoading] = useState(false);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const launcher_tour = useMemo(() => build_launcher_tour(t), [t]);
  usePageOnboardingTour({
    tour: launcher_tour,
    enabled: variant === "full",
    auto_start_delay_ms: 260,
  });
  const decorative_tokens = useMemo(
    () => build_decorative_tokens(agents, rooms),
    [agents, rooms],
  );

  const mention_targets = useMemo(
    () => build_launcher_mention_targets(agents, rooms),
    [agents, rooms],
  );

  const recent_entries = useMemo(
    () => build_recent_launcher_entries(conversations),
    [conversations],
  );

  const handle_open_recent_entry = useCallback(
    (entry: RecentLauncherEntry) => {
      void (async () => {
        try {
          if (entry.conversation_id) {
            if (!entry.room_id) {
              return;
            }
            set_active_panel_item(entry.room_id);
            on_open_route(
              AppRouteBuilders.room_conversation(entry.room_id, entry.conversation_id),
            );
            return;
          }

          if (entry.type === "dm" && entry.agent_id) {
            on_select_agent(entry.agent_id);
            const context = await ensure_direct_room(entry.agent_id);
            set_active_panel_item(context.room.id);
            on_open_route(
              AppRouteBuilders.room_conversation(context.room.id, context.conversation.id),
            );
            return;
          }

          if (!entry.room_id) {
            return;
          }

          const contexts = await get_room_contexts(entry.room_id);
          if (contexts.length > 0) {
            set_active_panel_item(entry.room_id);
            on_open_route(
              AppRouteBuilders.room_conversation(entry.room_id, contexts[0].conversation.id),
            );
          }
        } catch (error) {
          console.error("Failed to open recent entry:", error);
        }
      })();
    },
    [on_open_route, on_select_agent, set_active_panel_item],
  );

  const handle_submit = useCallback(
    async (next_query?: string) => {
      const trimmed = (next_query ?? query).trim();
      if (!trimmed || isQueryLoading) {
        return;
      }

      setIsQueryLoading(true);
      try {
        const action = await query_launcher({ query: trimmed });

        switch (action.action_type) {
          case "open_agent_dm": {
            on_select_agent(action.target_id);
            const context = await ensure_direct_room(action.target_id);
            if (context) {
              set_active_panel_item(context.room.id);
              const route = AppRouteBuilders.room_conversation(
                context.room.id,
                context.conversation.id,
              );
              const final_route = action.initial_message
                ? `${route}?initial=${encodeURIComponent(action.initial_message)}`
                : route;
              on_open_route(final_route);
            }
            break;
          }
          case "open_app": {
            on_open_main_agent_dm(action.initial_message || trimmed);
            break;
          }
          case "open_room": {
            const contexts = await get_room_contexts(action.target_id);
            if (contexts.length > 0) {
              set_active_panel_item(action.target_id);
              const route = AppRouteBuilders.room_conversation(
                action.target_id,
                contexts[0].conversation.id,
              );
              const final_route = action.initial_message
                ? `${route}?initial=${encodeURIComponent(action.initial_message)}`
                : route;
              on_open_route(final_route);
            }
            break;
          }
        }
      } catch (error) {
        console.error("Launcher query failed:", error);
      } finally {
        setIsQueryLoading(false);
      }
    },
    [
      query,
      isQueryLoading,
      on_open_main_agent_dm,
      on_open_route,
      on_select_agent,
      set_active_panel_item,
    ],
  );

  const handle_enter_home = useCallback(() => {
    on_open_route(AppRouteBuilders.home());
  }, [on_open_route]);

  const handle_input_change = useCallback((value: string) => {
    setQuery(value);
  }, []);

  const handle_primary_action = useCallback(
    (submitted_input: string) => {
      const trimmed_query = submitted_input.trim();
      if (!trimmed_query || isQueryLoading) {
        return false;
      }

      setQuery("");
      void handle_submit(trimmed_query);
      return true;
    },
    [handle_submit, isQueryLoading],
  );

  if (variant === "compact") {
    return (
      <CompactLauncherConsole
        agents={agents}
        is_query_loading={isQueryLoading}
        on_enter_home={handle_enter_home}
        on_open_recent_entry={handle_open_recent_entry}
        on_query_change={handle_input_change}
        on_select_agent={on_select_agent}
        on_submit={handle_primary_action}
        query={query}
        recent_entries={recent_entries}
        rooms={rooms}
      />
    );
  }

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
          <span
            className="text-[32px] font-semibold text-foreground mb-3"
            style={{
              fontFamily: '"striper", var(--font-sans)',
              fontWeight: 400,
            }}
          >
            nexus
          </span>
        </div>
      </div>
      <div
        className={cn(
          "relative flex min-h-0 flex-1 items-center justify-center px-8",
          "pb-8 pt-6",
        )}
      >
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

interface CompactLauncherEntry {
  key: string;
  title: string;
  subtitle: string;
  Icon: typeof Search;
  on_select: () => void;
}

interface CompactLauncherConsoleProps {
  agents: LauncherConsoleProps["agents"];
  rooms: LauncherConsoleProps["rooms"];
  recent_entries: RecentLauncherEntry[];
  query: string;
  is_query_loading: boolean;
  on_enter_home: () => void;
  on_open_recent_entry: (entry: RecentLauncherEntry) => void;
  on_query_change: (value: string) => void;
  on_select_agent: (agent_id: string) => void;
  on_submit: (submitted_query: string) => boolean;
}

function CompactLauncherConsole({
  agents,
  rooms,
  recent_entries,
  query,
  is_query_loading,
  on_enter_home,
  on_open_recent_entry,
  on_query_change,
  on_select_agent,
  on_submit,
}: CompactLauncherConsoleProps) {
  const { t } = useI18n();
  const input_ref = useRef<HTMLInputElement>(null);
  const [active_index, set_active_index] = useState(0);
  const trimmed_query = query.trim();

  useEffect(() => {
    requestAnimationFrame(() => {
      input_ref.current?.focus();
    });
  }, []);

  const entries = useMemo<CompactLauncherEntry[]>(() => {
    const base_entries: CompactLauncherEntry[] = [];

    if (trimmed_query) {
      base_entries.push({
        key: "query",
        title: trimmed_query,
        subtitle: t("launcher.compact_handoff"),
        Icon: CornerDownLeft,
        on_select: () => {
          on_submit(trimmed_query);
        },
      });
    } else {
      base_entries.push({
        key: "workspace",
        title: t("launcher.enter_app"),
        subtitle: t("launcher.compact_workspace"),
        Icon: Compass,
        on_select: on_enter_home,
      });
    }

    recent_entries.forEach((entry) => {
      base_entries.push({
        key: `recent-${entry.key}`,
        title: `${entry.type === "room" ? "#" : ""}${entry.label}`,
        subtitle: t("launcher.compact_recent"),
        Icon: Clock3,
        on_select: () => on_open_recent_entry(entry),
      });
    });

    agents.slice(0, 8).forEach((agent) => {
      base_entries.push({
        key: `agent-${agent.id}`,
        title: agent.name,
        subtitle: agent.description || "Agent",
        Icon: Bot,
        on_select: () => on_select_agent(agent.id),
      });
    });

    rooms
      .filter((room) => room.room_type === "room")
      .slice(0, 8)
      .forEach((room) => {
        base_entries.push({
          key: `room-${room.id}`,
          title: `#${room.name?.trim() || "未命名 Room"}`,
          subtitle: "Room",
          Icon: Hash,
          on_select: () => {
            on_open_recent_entry({
              key: `room-${room.id}`,
              type: "room",
              label: room.name?.trim() || "未命名 Room",
              last_activity_at: 0,
              room_id: room.id,
            });
          },
        });
      });

    const needle = trimmed_query.toLocaleLowerCase();
    if (!needle) {
      return base_entries.slice(0, 7);
    }

    return base_entries
      .filter((entry) => {
        if (entry.key === "query") {
          return true;
        }
        return `${entry.title} ${entry.subtitle}`.toLocaleLowerCase().includes(needle);
      })
      .slice(0, 7);
  }, [
    agents,
    on_enter_home,
    on_open_recent_entry,
    on_select_agent,
    on_submit,
    recent_entries,
    rooms,
    t,
    trimmed_query,
  ]);

  useEffect(() => {
    set_active_index(0);
  }, [query]);

  const selected_index = entries.length === 0
    ? -1
    : Math.min(active_index, entries.length - 1);

  const activate_entry = useCallback(
    (entry: CompactLauncherEntry | undefined) => {
      if (!entry || is_query_loading) {
        return;
      }
      entry.on_select();
    },
    [is_query_loading],
  );

  return (
    <section
      className="flex h-full min-h-0 flex-1 items-start justify-center px-3 py-3 text-foreground"
      onClick={(event) => event.stopPropagation()}
    >
      <div
        className="flex w-full max-w-[640px] flex-col overflow-hidden rounded-[18px] border"
        style={{
          background:
            "linear-gradient(180deg, var(--launcher-input-fill), var(--launcher-input-inner-fill))",
          borderColor: "var(--launcher-input-stroke)",
          boxShadow:
            "inset 0 1px 0 var(--launcher-input-inner-stroke), 0 20px 48px rgba(36, 50, 78, 0.14)",
        }}
      >
        <div className="flex h-16 items-center gap-3 border-b border-(--divider-subtle-color) px-4">
          <Search
            className="h-5 w-5 shrink-0"
            style={{ color: "var(--launcher-input-icon)" }}
          />
          <input
            ref={input_ref}
            className="min-w-0 flex-1 bg-transparent text-[16px] font-medium outline-none placeholder:text-(--launcher-input-placeholder)"
            disabled={is_query_loading}
            onChange={(event) => on_query_change(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                set_active_index((index) => Math.min(index + 1, entries.length - 1));
                return;
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                set_active_index((index) => Math.max(index - 1, 0));
                return;
              }
              if (event.key === "Enter") {
                event.preventDefault();
                activate_entry(entries[selected_index]);
              }
            }}
            placeholder={t("launcher.compact_placeholder")}
            style={{ color: "var(--launcher-input-text)" }}
            value={query}
          />
          <kbd className="rounded-md border border-(--divider-subtle-color) bg-(--surface-inset-background) px-1.5 py-1 text-[10px] font-semibold text-(--text-soft)">
            Esc
          </kbd>
        </div>

        <div className="max-h-[330px] overflow-y-auto p-2">
          {entries.length > 0 ? (
            entries.map((entry, index) => {
              const Icon = entry.Icon;
              const is_active = index === selected_index;
              return (
                <button
                  className={cn(
                    "flex w-full min-w-0 items-center gap-3 rounded-xl px-3 py-2.5 text-left transition duration-(--motion-duration-fast)",
                    is_active
                      ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                      : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                  )}
                  key={entry.key}
                  onClick={() => activate_entry(entry)}
                  onMouseEnter={() => set_active_index(index)}
                  type="button"
                >
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-(--divider-subtle-color) bg-(--surface-inset-background) text-(--icon-default)">
                    <Icon className="h-4 w-4" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-semibold">
                      {entry.title}
                    </span>
                    <span className="block truncate text-[11px] text-(--text-soft)">
                      {entry.subtitle}
                    </span>
                  </span>
                  {is_active ? (
                    <CornerDownLeft className="h-4 w-4 shrink-0 text-(--icon-muted)" />
                  ) : null}
                </button>
              );
            })
          ) : (
            <div className="px-3 py-8 text-center text-sm text-(--text-soft)">
              {t("launcher.compact_empty")}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
