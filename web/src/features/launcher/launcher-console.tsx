"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowRight, ArrowUp, MessageSquare } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { AppRouteBuilders } from "@/app/router/route-paths";

import {
  HeroBlobShell,
} from "@/features/launcher/launcher-glass-shell";
import { cn } from "@/lib/utils";
import { ANIMATIONS } from "@/config/animation-assets";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { useSidebarStore } from "@/store/sidebar";
import { Agent } from "@/types/agent/agent";
import { Conversation } from "@/types/conversation/conversation";
import { ConversationWithOwner, SpotlightToken } from "@/types/app/launcher";
import { RoomAggregate } from "@/types/conversation/room";
import { query_launcher } from "@/lib/api/launcher-api";
import { ensure_direct_room, get_room_contexts } from "@/lib/api/room-api";
import { parse_session_key } from "@/lib/conversation/session-key";
import { MentionTargetItem, MentionTargetPopover } from "@/features/conversation/shared/mention-popover";

import { AgentPile } from "./launcher-agent-pile";
import { AnimatedHeroText, FadeSlideIn } from "@/shared/ui/feedback/animated-hero-text";

interface LauncherConsoleProps {
  agents: Agent[];
  conversations: Conversation[];
  rooms: RoomAggregate[];
  current_agent_id: string | null;
  on_open_main_agent_dm: (initial_prompt?: string) => void;
  on_select_agent: (agent_id: string) => void;
}

interface HeroStageProps {
  current_agent_id: string | null;
  decorative_tokens: SpotlightToken[];
  mention_targets: MentionTargetItem[];
  on_enter_home: () => void;
  on_open_main_agent_dm: (initial_prompt?: string) => void;
  on_query_change: (value: string) => void;
  on_select_agent: (agent_id: string) => void;
  on_open_recent_entry: (entry: RecentLauncherEntry) => void;
  on_submit: (submitted_query: string) => boolean;
  query: string;
  recent_entries: RecentLauncherEntry[];
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

interface LauncherMentionMatch {
  trigger: "@" | "#";
  filter: string;
  start_pos: number;
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

function get_initials(name: string) {
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

function truncate_launcher_chip_label(label: string, max_chars: number = 8): string {
  const chars = Array.from(label.trim());
  if (chars.length <= max_chars) {
    return label.trim();
  }

  // Hero 推荐项空间很窄，超长名称改为中间省略。
  const head_count = Math.max(2, Math.ceil((max_chars - 1) / 2));
  const tail_count = Math.max(2, max_chars - 1 - head_count);
  return `${chars.slice(0, head_count).join("")}…${chars.slice(-tail_count).join("")}`;
}

function is_launcher_chip_truncated(label: string, max_chars: number = 6): boolean {
  return Array.from(label.trim()).length > max_chars;
}

function build_decorative_tokens(
  agents: Agent[],
  conversations_with_owners: ConversationWithOwner[],
): SpotlightToken[] {
  const agent_tokens: SpotlightToken[] =
    agents.map((agent, index) => ({
      key: `agent-${agent.agent_id}`,
      label: get_initials(agent.name),
      agent_id: agent.agent_id,
      kind: "agent" as const,
      swatch: TOKEN_SWATCHES[index % TOKEN_SWATCHES.length],
    }));

  const room_tokens: SpotlightToken[] =
    conversations_with_owners.slice(0, 8).map(({ conversation }, index) => ({
      key: `room-${conversation.session_key}`,
      label: get_initials(conversation.title || "Room"),
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
    if (source.length < 18) {
      source.push({
        key: `fallback-${item.label}-${index}`,
        label: item.label,
        agent_id: null,
        kind: item.kind,
        swatch: TOKEN_SWATCHES[(agent_tokens.length + room_tokens.length + index) % TOKEN_SWATCHES.length],
      });
    }
  });

  return source.slice(0, 12);
}

function build_launcher_mention_targets(
  agents: Agent[],
  rooms: RoomAggregate[],
  conversations_with_owners: ConversationWithOwner[],
): MentionTargetItem[] {
  const agent_targets = agents
    .slice()
    .sort((left, right) => left.name.localeCompare(right.name, "zh-CN"))
    .map((agent) => ({
      id: `agent-${agent.agent_id}`,
      label: agent.name,
      subtitle: "Agent",
      kind: "agent" as const,
    }));

  const room_name_map = new Map(
    rooms
      .filter((room) => room.room.room_type === "room")
      .map((room) => [room.room.id, room.room.name?.trim() || "未命名 Room"]),
  );
  const latest_room_by_id = new Map<string, MentionTargetItem & { last_activity_at: number }>();
  for (const { conversation } of conversations_with_owners) {
    if (!conversation.room_id || !conversation.conversation_id) {
      continue;
    }
    const parsed_session_key = parse_session_key(conversation.session_key);
    if (parsed_session_key.chat_type !== "group") {
      continue;
    }
    const existing_room = latest_room_by_id.get(conversation.room_id);
    if (existing_room && existing_room.last_activity_at >= conversation.last_activity_at) {
      continue;
    }
    latest_room_by_id.set(conversation.room_id, {
      id: `room-${conversation.room_id}`,
      label: room_name_map.get(conversation.room_id) || "未命名 Room",
      subtitle: "Room",
      kind: "room",
      last_activity_at: conversation.last_activity_at,
    });
  }

  const room_targets = Array.from(latest_room_by_id.values())
    .sort((left, right) => right.last_activity_at - left.last_activity_at)
    .map(({ last_activity_at: _, ...item }) => item);

  return [...agent_targets, ...room_targets];
}

function find_launcher_mention_match(
  value: string,
  cursor_pos: number,
): LauncherMentionMatch | null {
  const before_cursor = value.slice(0, cursor_pos);
  const match = before_cursor.match(/(?:^|\s)([@#])([^\s@#]*)$/);
  if (!match) {
    return null;
  }
  const trigger = match[1] as "@" | "#";
  const filter = match[2] ?? "";
  const start_pos = before_cursor.length - filter.length - 1;
  return {
    trigger,
    filter,
    start_pos,
  };
}

const MemoAgentPile = memo(AgentPile);

function build_recent_launcher_entries(
  conversations_with_owners: ConversationWithOwner[],
): RecentLauncherEntry[] {
  const latest_dm_by_agent = new Map<string, RecentLauncherEntry>();
  const latest_room_by_id = new Map<string, RecentLauncherEntry>();

  for (const { conversation, owner } of conversations_with_owners) {
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
  mention_targets,
  on_enter_home,
  on_open_main_agent_dm,
  on_query_change,
  on_select_agent,
  on_open_recent_entry,
  on_submit,
  query,
  recent_entries,
  is_query_loading,
}: HeroStageProps) {
  const { t } = useI18n();
  const is_composing_ref = useRef(false);
  const input_ref = useRef<HTMLInputElement>(null);
  const [local_query, set_local_query] = useState(query);
  const [mention_match, set_mention_match] = useState<LauncherMentionMatch | null>(null);

  const visible_mention_targets = useMemo(() => {
    if (!mention_match) {
      return [];
    }
    return mention_targets.filter((item) => (
      mention_match.trigger === "@"
        ? item.kind === "agent"
        : item.kind === "room"
    ));
  }, [mention_match, mention_targets]);

  const sync_mention_match = useCallback((value: string, cursor_pos: number) => {
    set_mention_match(find_launcher_mention_match(value, cursor_pos));
  }, []);

  const handle_mention_close = useCallback(() => {
    set_mention_match(null);
  }, []);

  const handle_enter_home_click = useCallback((event: React.MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    on_enter_home();
  }, [on_enter_home]);

  const handle_query_change = useCallback((value: string) => {
    set_local_query(value);
    on_query_change(value);
    const cursor_pos = input_ref.current?.selectionStart ?? value.length;
    sync_mention_match(value, cursor_pos);
  }, [on_query_change, sync_mention_match]);

  const handle_mention_select = useCallback((item: MentionTargetItem) => {
    if (!mention_match) {
      return;
    }
    const cursor_pos = input_ref.current?.selectionStart ?? local_query.length;
    const before = local_query.slice(0, mention_match.start_pos);
    const after = local_query.slice(cursor_pos);
    const next_query = `${before}${mention_match.trigger}${item.label} ${after}`;
    set_local_query(next_query);
    on_query_change(next_query);
    set_mention_match(null);

    requestAnimationFrame(() => {
      const next_cursor = mention_match.start_pos + item.label.length + 2;
      input_ref.current?.setSelectionRange(next_cursor, next_cursor);
      input_ref.current?.focus();
    });
  }, [local_query, mention_match, on_query_change]);

  useEffect(() => {
    set_local_query(query);
  }, [query]);

  useEffect(() => {
    if (!local_query) {
      set_mention_match(null);
    }
  }, [local_query]);

  const handle_submit = useCallback(() => {
    const trimmed_query = local_query.trim();
    if (!trimmed_query) {
      return;
    }

    const did_submit = on_submit(trimmed_query);
    if (!did_submit) {
      return;
    }

    // 提交后先在本地立即清空，避免受控值回流慢一拍。
    set_local_query("");
    on_query_change("");
    set_mention_match(null);
  }, [local_query, on_query_change, on_submit]);

  return (
    <div className="relative z-10 flex w-full max-w-[1180px] flex-col items-center" onClick={(e) => e.stopPropagation()}>
      <HeroBlobShell class_name="z-10 transition-transform duration-500 ease-out">
        <div className="space-y-3 sm:space-y-4">
          <FadeSlideIn delay_ms={0} duration_ms={380} y_offset={6}>
            <div className="flex flex-col items-center gap-2.5">
              <button
                className="group inline-flex items-center gap-3 rounded-full px-2 py-2 pr-4 text-left transition duration-200 ease-out hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/45"
                style={{
                  background: "color-mix(in srgb, var(--launcher-input-fill) 92%, rgba(255, 255, 255, 0.12))",
                  boxShadow: "inset 0 0 0 1px var(--launcher-input-stroke), 0 12px 26px rgba(48, 63, 88, 0.10)",
                  color: "var(--launcher-input-text)",
                }}
                onClick={handle_enter_home_click}
                type="button"
              >
                <span
                  className="inline-flex min-h-8 items-center justify-center rounded-full px-3 text-[10px] font-semibold tracking-[0.22em]"
                  style={{
                    background: "color-mix(in srgb, var(--launcher-input-inner-fill) 68%, rgba(255, 255, 255, 0.34))",
                    boxShadow: "inset 0 0 0 1px var(--launcher-input-inner-stroke)",
                  }}
                >
                  APP
                </span>
                <span className="text-[12px] font-semibold tracking-[0.12em] text-foreground/90 sm:text-[13px]">
                  {t("launcher.enter_app")}
                </span>
                <ArrowRight className="h-3.5 w-3.5 transition-transform duration-200 ease-out group-hover:translate-x-0.5" />
              </button>
            </div>
          </FadeSlideIn>
          <div className="relative inline-block">
            <LottiePlayer
              class_name="pointer-events-none absolute -right-4 -top-5 h-12 w-12 opacity-[0.46] sm:-right-16 sm:-top-14 sm:h-24 sm:w-24"
              inline_style={undefined}
              src={ANIMATIONS.SPARKLES}
            />
            <h1 className="mb-2 text-[24px] font-extrabold leading-[1.12] tracking-[-0.05em] text-foreground/96 sm:text-[42px] sm:leading-[1.05]">
              <AnimatedHeroText text={t("launcher.hero_title")} initial_delay_ms={80} stagger_ms={26} />
            </h1>
          </div>
        </div>

        <div className="mt-8 sm:mt-10">
          <FadeSlideIn delay_ms={440} duration_ms={420} y_offset={10}>
            <div
              className="mx-auto w-full max-w-[326px] rounded-2xl border px-4 py-1 sm:max-w-[420px] "
              style={{
                background: "linear-gradient(180deg, var(--launcher-input-fill), var(--launcher-input-inner-fill))",
                borderColor: "var(--launcher-input-stroke)",
                boxShadow: "inset 0 1px 0 var(--launcher-input-inner-stroke), 0 14px 30px rgba(56, 72, 98, 0.10)",
              }}
            >
              <div className="relative flex min-w-0 items-center gap-2.5 sm:gap-3">
                {mention_match ? (
                  <MentionTargetPopover
                    anchor_rect={input_ref.current?.getBoundingClientRect() ?? null}
                    filter={mention_match.filter}
                    items={visible_mention_targets}
                    on_close={handle_mention_close}
                    on_select={handle_mention_select}
                    placement="below"
                  />
                ) : null}
                <MessageSquare className="h-4.5 w-4.5" style={{ color: "var(--launcher-input-icon)" }} />
                <input
                  ref={input_ref}
                  className="flex-1 bg-transparent text-[14px] outline-none shadow-none ring-0 placeholder:text-(--launcher-input-placeholder) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none sm:text-[15px]"
                  style={{ color: "var(--launcher-input-text)" }}
                  onBlur={() => {
                    requestAnimationFrame(() => {
                      if (document.activeElement !== input_ref.current) {
                        set_mention_match(null);
                      }
                    });
                  }}
                  onChange={(event) => handle_query_change(event.target.value)}
                  onCompositionEnd={() => {
                    is_composing_ref.current = false;
                  }}
                  onCompositionStart={() => {
                    is_composing_ref.current = true;
                  }}
                  onKeyDown={(event) => {
                    if (is_composing_ref.current || event.nativeEvent.isComposing) {
                      return;
                    }
                    if (mention_match && ["ArrowDown", "ArrowUp", "Enter", "Tab", "Escape"].includes(event.key)) {
                      return;
                    }
                    if (event.key === "Enter") {
                      event.preventDefault();
                      handle_submit();
                    }
                  }}
                  onSelect={(event) => {
                    const target = event.target as HTMLInputElement;
                    sync_mention_match(target.value, target.selectionStart ?? target.value.length);
                  }}
                  value={local_query}
                  placeholder={t("launcher.query_placeholder")}
                  disabled={is_query_loading}
                />
                <button
                  className={cn(
                    "inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full border transition duration-150 ease-out hover:-translate-y-0.5 sm:h-11 sm:w-11",
                    is_query_loading && "cursor-not-allowed opacity-(--disabled-opacity) hover:translate-y-0",
                  )}
                  style={{
                    background: "var(--launcher-submit-background)",
                    borderColor: "rgba(255,255,255,0.34)",
                    boxShadow: "inset 0 1px 0 rgba(255,255,255,0.26), var(--launcher-submit-shadow)",
                    color: "var(--launcher-submit-color)",
                  }}
                  onClick={handle_submit}
                  type="button"
                  disabled={is_query_loading}
                >
                  {is_query_loading ? (
                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-(--divider-strong-color) border-t-transparent" />
                  ) : (
                    <ArrowUp className="h-4 w-4" />
                  )}
                </button>
              </div>
            </div>
          </FadeSlideIn>

          <div className={cn(
            "flex flex-wrap items-center justify-center gap-2",
            "mt-3 sm:mt-4",
          )}>
            {recent_entries.map((entry, index) => (
              <FadeSlideIn key={entry.key} delay_ms={580 + index * 55} duration_ms={360} y_offset={6} style={{ display: "inline-flex" }}>
                <div className="group relative inline-flex">
                  {is_launcher_chip_truncated(entry.label) ? (
                    <div
                      className="pointer-events-none absolute left-1/2 top-full z-20 mt-2 w-max max-w-[220px] -translate-x-1/2 translate-y-1 rounded-2xl px-3 py-2 text-center text-xs font-medium leading-5 opacity-0 shadow-[0_18px_42px_rgba(38,52,76,0.16)] transition duration-200 ease-out group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:translate-y-0 group-focus-within:opacity-100"
                      style={{
                        background: "rgba(247, 249, 253, 0.96)",
                        boxShadow: "0 18px 42px rgba(38, 52, 76, 0.16), inset 0 0 0 1px rgba(255, 255, 255, 0.52)",
                        color: "rgba(39, 50, 74, 0.88)",
                      }}
                    >
                      {entry.type === "room" ? "#" : ""}
                      {entry.label}
                    </div>
                  ) : null}
                  <button
                    aria-label={entry.type === "room" ? `房间 ${entry.label}` : `私聊 ${entry.label}`}
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
                    {truncate_launcher_chip_label(entry.label)}
                  </button>
                </div>
              </FadeSlideIn>
            ))}

            <FadeSlideIn delay_ms={580 + recent_entries.length * 55} duration_ms={360} y_offset={6} style={{ display: "inline-flex" }}>
              <button
                className="px-2 text-xs font-medium transition-colors duration-150 ease-out hover:text-(--launcher-handoff-hover-color) sm:text-sm"
                style={{ color: "var(--launcher-handoff-color)" }}
                onClick={() => on_open_main_agent_dm(query)}
                type="button"
              >
                <span className="inline-flex items-center gap-1.5">
                  {t("launcher.handoff")}
                  <ArrowRight className="h-3.5 w-3.5" />
                </span>
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
        <HeroStage
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
