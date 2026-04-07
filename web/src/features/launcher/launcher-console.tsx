"use client";

import { memo, useCallback, useMemo, useState } from "react";
import { ArrowUp, MessageSquare } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { AppRouteBuilders } from "@/app/router/route-paths";

import {
  HeroActionOrbShell,
  HeroBlobShell,
  HeroInputShell,
} from "@/features/launcher/launcher-glass-shell";
import { cn, truncate } from "@/lib/utils";
import { ANIMATIONS } from "@/config/animation-assets";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { useSidebarStore } from "@/store/sidebar";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { ConversationWithOwner, SpotlightToken } from "@/types/launcher";
import { queryLauncher } from "@/lib/launcher-api";
import { ensureDirectRoom, getRoomContexts } from "@/lib/room-api";
import { parseSessionKey } from "@/lib/session-key";

import { AgentPile } from "./launcher-agent-pile";
import { AnimatedHeroText, FadeSlideIn } from "@/shared/ui/feedback/animated-hero-text";

interface LauncherConsoleProps {
  agents: Agent[];
  conversations: Conversation[];
  current_agent_id: string | null;
  on_open_app_conversation: (initial_prompt?: string) => void;
  on_close_app_conversation: () => void;
  is_app_conversation_open: boolean;
  on_select_agent: (agent_id: string) => void;
  surface: "launcher" | "app";
}

interface HeroStageProps {
  current_agent_id: string | null;
  decorative_tokens: SpotlightToken[];
  on_open_app_conversation: (initial_prompt?: string) => void;
  on_close_app_conversation: () => void;
  is_app_conversation_open: boolean;
  on_query_change: (value: string) => void;
  on_select_agent: (agent_id: string) => void;
  on_open_recent_entry: (entry: RecentLauncherEntry) => void;
  on_submit: () => void;
  query: string;
  recent_entries: RecentLauncherEntry[];
  surface: "launcher" | "app";
  is_query_loading: boolean;
}

interface RecentLauncherEntry {
  key: string;
  type: "dm" | "room";
  label: string;
  last_activity_at: number;
  agent_id?: string;
  room_id?: string;
  conversation_id?: string;
}


const TOKEN_SWATCHES = [
  { fill: "#5FA052", text: "#FFFFFF", ring: "#8DBA86" },
  { fill: "#E8A838", text: "#FFFFFF", ring: "#F0C56C" },
  { fill: "#4DAA9F", text: "#FFFFFF", ring: "#7CC8BE" },
  { fill: "#A78BFA", text: "#FFFFFF", ring: "#C2B0FF" },
  { fill: "#6C7BDB", text: "#FFFFFF", ring: "#9AA4F2" },
  { fill: "#D4687A", text: "#FFFFFF", ring: "#E597A3" },
  { fill: "#C4A86B", text: "#FFFFFF", ring: "#D7C08D" },
  { fill: "#8B9089", text: "#FFFFFF", ring: "#B6BAB4" },
  { fill: "#E8945A", text: "#FFFFFF", ring: "#F0B186" },
];

function getInitials(name: string) {
  const parts = name
    .trim()
    .split(/\s+/)
    .filter(Boolean);
  if (parts.length === 0) {
    return "AG";
  }
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return `${parts[0][0] ?? ""}${parts[1][0] ?? ""}`.toUpperCase();
}

function buildDecorativeTokens(
  agents: Agent[],
  conversations_with_owners: ConversationWithOwner[],
): SpotlightToken[] {
  const agent_tokens: SpotlightToken[] =
    agents.map((agent, index) => ({
      key: `agent-${agent.agent_id}`,
      label: getInitials(agent.name),
      agent_id: agent.agent_id,
      kind: "agent" as const,
      swatch: TOKEN_SWATCHES[index % TOKEN_SWATCHES.length],
    }));

  const room_tokens: SpotlightToken[] =
    conversations_with_owners.slice(0, 8).map(({ conversation }, index) => ({
      key: `room-${conversation.session_key}`,
      label: getInitials(conversation.title || "Room"),
      agent_id: conversation.agent_id ?? null,
      kind: "room" as const,
      swatch: TOKEN_SWATCHES[(agent_tokens.length + index) % TOKEN_SWATCHES.length],
    }));

  const fallback = [
    { label: "SA", kind: "agent" as const },
    { label: "NV", kind: "agent" as const },
    { label: "BO", kind: "agent" as const },
    { label: "DX", kind: "room" as const },
    { label: "WR", kind: "room" as const },
    { label: "QA", kind: "room" as const },
    { label: "SP", kind: "room" as const },
    { label: "AR", kind: "room" as const },
    { label: "NO", kind: "agent" as const },
    { label: "PR", kind: "agent" as const },
    { label: "FL", kind: "agent" as const },
    { label: "PI", kind: "agent" as const },
    { label: "RL", kind: "room" as const },
    { label: "AT", kind: "agent" as const },
  ];

  const source: SpotlightToken[] = [
    ...agent_tokens,
    ...room_tokens,
  ];
  fallback.forEach((item, index) => {
    if (source.length < 22) {
      source.push({
        key: `fallback-${item.label}-${index}`,
        label: item.label,
        agent_id: null,
        kind: item.kind,
        swatch: TOKEN_SWATCHES[(agent_tokens.length + room_tokens.length + index) % TOKEN_SWATCHES.length],
      });
    }
  });

  return source.slice(0, 22);
}

const MemoAgentPile = memo(AgentPile);

function buildRecentLauncherEntries(
  conversations_with_owners: ConversationWithOwner[],
): RecentLauncherEntry[] {
  const latest_dm_by_agent = new Map<string, RecentLauncherEntry>();
  const latest_room_by_id = new Map<string, RecentLauncherEntry>();

  for (const { conversation, owner } of conversations_with_owners) {
    const parsed_session_key = parseSessionKey(conversation.session_key);
    if (parsed_session_key.ref?.startsWith("launcher-app-")) {
      continue;
    }

    if (conversation.room_id && conversation.conversation_id) {
      const existing_room = latest_room_by_id.get(conversation.room_id);
      if (!existing_room || conversation.last_activity_at > existing_room.last_activity_at) {
        latest_room_by_id.set(conversation.room_id, {
          key: `room-${conversation.room_id}`,
          type: "room",
          room_id: conversation.room_id,
          conversation_id: conversation.conversation_id,
          label: conversation.title || "未命名 Room",
          last_activity_at: conversation.last_activity_at,
        });
      }
      continue;
    }

    if (!conversation.agent_id || !owner) {
      continue;
    }

    const existing_dm = latest_dm_by_agent.get(conversation.agent_id);
    if (!existing_dm || conversation.last_activity_at > existing_dm.last_activity_at) {
      latest_dm_by_agent.set(conversation.agent_id, {
        key: `dm-${conversation.agent_id}`,
        type: "dm",
        agent_id: conversation.agent_id,
        label: owner.name,
        last_activity_at: conversation.last_activity_at,
      });
    }
  }

  return [
    ...Array.from(latest_dm_by_agent.values()),
    ...Array.from(latest_room_by_id.values()),
  ]
    .sort((left, right) => right.last_activity_at - left.last_activity_at)
    .slice(0, 3);
}

const HeroStage = memo(function HeroStage({
  current_agent_id,
  decorative_tokens,
  on_open_app_conversation,
  on_close_app_conversation,
  is_app_conversation_open,
  on_query_change,
  on_select_agent,
  on_open_recent_entry,
  on_submit,
  query,
  recent_entries,
  surface,
  is_query_loading,
}: HeroStageProps) {
  const { t } = useI18n();

  return (
    <div className="relative flex w-full max-w-[1180px] flex-col items-center" onClick={(e) => e.stopPropagation()}>
      <HeroBlobShell
        class_name={cn(
          "z-10 transition-transform duration-500 ease-out lg:origin-left",
          surface === "app" && "lg:translate-x-[-4%] lg:scale-[0.97]",
        )}
      >
        <div className="space-y-3">
          <FadeSlideIn delay_ms={0} duration_ms={380} y_offset={6}>
            <p className="text-[9px] font-medium uppercase tracking-[0.32em] text-muted-foreground/70">
              {t("launcher.collaboration_hub")}
            </p>
          </FadeSlideIn>
          <div className="relative inline-block">
            <LottiePlayer
              class_name="pointer-events-none absolute -right-4 -top-5 h-12 w-12 opacity-[0.46] sm:-right-16 sm:-top-14 sm:h-24 sm:w-24"
              inline_style={undefined}
              src={ANIMATIONS.SPARKLES}
            />
            <h1
              className="mb-7 text-[24px] font-extrabold leading-[1.12] tracking-[-0.05em] text-foreground/96 sm:mb-10 sm:text-[42px] sm:leading-[1.05]">
              <AnimatedHeroText text={t("launcher.hero_title")} initial_delay_ms={80} stagger_ms={26} />
            </h1>
          </div>
        </div>

        <div className="mt-3 sm:mt-4">
          <FadeSlideIn delay_ms={440} duration_ms={420} y_offset={10}>
            <HeroInputShell class_name="mx-auto w-full max-w-[326px] sm:max-w-[480px]">
              <div className="flex min-w-0 items-center gap-2.5 sm:gap-3">
                <MessageSquare
                  className="h-4.5 w-4.5"
                  style={{ color: "var(--launcher-input-icon)" }}
                />
                <input
                  className="flex-1 bg-transparent text-[14px] outline-none placeholder:text-[color:var(--launcher-input-placeholder)] sm:text-[15px]"
                  style={{
                    color: "var(--launcher-input-text)",
                  }}
                  onChange={(event) => on_query_change(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      on_submit();
                    }
                  }}
                  placeholder={t("launcher.query_placeholder")}
                  value={query}
                  disabled={is_query_loading}
                />
                <HeroActionOrbShell class_name="shrink-0" is_active={!is_query_loading}>
                  <button
                    className={cn(
                      "inline-flex h-full w-full items-center justify-center rounded-full transition duration-150 ease-out hover:-translate-y-0.5",
                      is_query_loading && "cursor-not-allowed opacity-50 hover:translate-y-0",
                    )}
                    style={{
                      background: "var(--launcher-submit-background)",
                      boxShadow: "var(--launcher-submit-shadow)",
                      color: "var(--launcher-submit-color)",
                    }}
                    onClick={on_submit}
                    type="button"
                    disabled={is_query_loading}
                  >
                    {is_query_loading ? (
                      <div className="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-transparent" />
                    ) : (
                      <ArrowUp className="h-4 w-4" />
                    )}
                  </button>
                </HeroActionOrbShell>
              </div>
            </HeroInputShell>
          </FadeSlideIn>

          <div className="mt-3 flex flex-wrap items-center justify-center gap-2 sm:mt-4">
            {recent_entries.map((entry, index) => (
              <FadeSlideIn key={entry.key} delay_ms={580 + index * 55} duration_ms={360} y_offset={6} style={{ display: "inline-flex" }}>
                <button
                  className="inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-xs font-medium transition duration-150 ease-out hover:-translate-y-0.5 sm:text-sm"
                  style={{
                    background: entry.type === "room"
                      ? "var(--launcher-room-chip-background)"
                      : "var(--launcher-agent-chip-background)",
                    boxShadow: entry.type === "room"
                      ? "inset 0 0 0 1px var(--launcher-room-chip-border)"
                      : "inset 0 0 0 1px var(--launcher-agent-chip-border)",
                    color: entry.type === "room"
                      ? "var(--launcher-room-chip-text)"
                      : "var(--launcher-agent-chip-text)",
                  }}
                  onClick={(event) => {
                    event.stopPropagation();
                    on_open_recent_entry(entry);
                  }}
                  type="button"
                >
                  {entry.type === "dm" ? (
                    <span
                      className="h-4 w-4 rounded-full"
                      style={{
                        backgroundColor: index === 0 ? "#bff0ca" : "#ffd7b8",
                        border: `1px solid ${index === 0 ? "#7fe3a8" : "#e3c6ad"}`,
                      }}
                    />
                  ) : null}
                  {entry.type === "room" ? "#" : ""}
                  {truncate(entry.label, 18)}
                </button>
              </FadeSlideIn>
            ))}

            <FadeSlideIn delay_ms={580 + recent_entries.length * 55} duration_ms={360} y_offset={6} style={{ display: "inline-flex" }}>
              <button
                className="px-2 text-xs font-medium transition-colors duration-150 ease-out hover:text-[color:var(--launcher-handoff-hover-color)] sm:text-sm"
                style={{ color: "var(--launcher-handoff-color)" }}
                onClick={() => is_app_conversation_open ? on_close_app_conversation() : on_open_app_conversation(query)}
                type="button"
              >
                {t("launcher.handoff")} →
              </button>
            </FadeSlideIn>
          </div>
        </div>
      </HeroBlobShell>

      <MemoAgentPile
        class_name="hidden min-[400px]:block"
        current_agent_id={current_agent_id}
        on_select_agent={on_select_agent}
        tokens={decorative_tokens}
      />
    </div>
  );
});

export function LauncherConsole({
  agents,
  conversations,
  current_agent_id,
  on_open_app_conversation,
  on_close_app_conversation,
  is_app_conversation_open,
  on_select_agent,
  surface,
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
    () => buildDecorativeTokens(agents, conversations_with_owners),
    [agents, conversations_with_owners],
  );

  const recent_entries = useMemo(
    () => buildRecentLauncherEntries(conversations_with_owners),
    [conversations_with_owners],
  );

  const handle_open_recent_entry = useCallback((entry: RecentLauncherEntry) => {
    void (async () => {
      try {
        if (entry.type === "dm" && entry.agent_id) {
          on_select_agent(entry.agent_id);
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

        const contexts = await getRoomContexts(entry.room_id);
        if (contexts.length > 0) {
          set_active_panel_item(entry.room_id);
          navigate(AppRouteBuilders.room_conversation(entry.room_id, contexts[0].conversation.id));
        }
      } catch (error) {
        console.error("Failed to open recent entry:", error);
      }
    })();
  }, [navigate, on_select_agent, set_active_panel_item]);

  const handle_submit = useCallback(async () => {
    const trimmed = query.trim();
    if (!trimmed || isQueryLoading) {
      return;
    }

    setIsQueryLoading(true);
    try {
      const action = await queryLauncher({ query: trimmed });

      switch (action.action_type) {
        case "open_agent_dm": {
          const context = await ensureDirectRoom(action.target_id);
          if (context) {
            set_active_panel_item(context.room.id);
            const route = AppRouteBuilders.room_conversation(context.room.id, context.conversation.id);
            // 如果有初始消息，在导航 URL 中编码以供 Room 页面使用
            const finalRoute = action.initial_message
              ? `${route}?initial=${encodeURIComponent(action.initial_message)}`
              : route;
            navigate(finalRoute);
          }
          break;
        }
        case "open_room": {
          const contexts = await getRoomContexts(action.target_id);
          if (contexts.length > 0) {
            set_active_panel_item(action.target_id);
            const route = AppRouteBuilders.room_conversation(action.target_id, contexts[0].conversation.id);
            // 如果有初始消息，在导航 URL 中编码以供 Room 页面使用
            const finalRoute = action.initial_message
              ? `${route}?initial=${encodeURIComponent(action.initial_message)}`
              : route;
            navigate(finalRoute);
          }
          break;
        }
        case "open_app":
          on_open_app_conversation(action.initial_message);
          break;
      }
    } catch (error) {
      console.error("Launcher query failed:", error);
    } finally {
      setIsQueryLoading(false);
    }
  }, [query, isQueryLoading, on_open_app_conversation, navigate, set_active_panel_item]);

  return (
    <section className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="relative z-30 flex items-center justify-between gap-3 px-3 pt-3 sm:px-7 sm:pt-1"
        onClick={(e) => e.stopPropagation()}>
        <div className="relative flex items-center gap-1 px-1 py-1">
          <LottiePlayer
            class_name="pointer-events-none absolute left-10 -top-4 h-12 w-12 opacity-[0.72] sm:left-3 sm:-top-12 sm:h-24 sm:w-24"
            inline_style={undefined}
            src={ANIMATIONS.BOM}
          />
          <img alt="" className="h-9 w-9 sm:h-10 sm:w-10" src="/logo.webp" />
          <span className="text-sm font-semibold text-foreground sm:text-base">Nexus</span>
        </div>

      </div>

      <div className="relative flex min-h-0 flex-1 items-center justify-center px-8 pb-8 pt-6">
        <HeroStage
          current_agent_id={current_agent_id}
          decorative_tokens={decorative_tokens}
          on_open_app_conversation={on_open_app_conversation}
          on_close_app_conversation={on_close_app_conversation}
          is_app_conversation_open={is_app_conversation_open}
          on_query_change={setQuery}
          on_select_agent={on_select_agent}
          on_open_recent_entry={handle_open_recent_entry}
          on_submit={handle_submit}
          query={query}
          recent_entries={recent_entries}
          surface={surface}
          is_query_loading={isQueryLoading}
        />
      </div>
    </section>
  );
}
