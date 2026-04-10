"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, ArrowRight, ArrowUp, MessageSquare, RotateCcw } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { AppRouteBuilders } from "@/app/router/route-paths";

import {
  HeroActionOrbShell,
  HeroBlobShell,
  HeroInputShell,
} from "@/features/launcher/launcher-glass-shell";
import { cn } from "@/lib/utils";
import { ANIMATIONS } from "@/config/animation-assets";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { useSidebarStore } from "@/store/sidebar";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { ConversationWithOwner, SpotlightToken } from "@/types/launcher";
import { RoomAggregate } from "@/types/room";
import { queryLauncher } from "@/lib/launcher-api";
import { ensureDirectRoom, getRoomContexts } from "@/lib/room-api";
import { parseSessionKey } from "@/lib/session-key";
import { MentionTargetItem, MentionTargetPopover } from "@/features/conversation-shared/mention-popover";

import { AgentPile } from "./launcher-agent-pile";
import { AnimatedHeroText, FadeSlideIn } from "@/shared/ui/feedback/animated-hero-text";

interface LauncherConsoleProps {
  app_conversation_draft: string;
  app_conversation_loading: boolean;
  app_conversation_can_control: boolean;
  app_conversation_control_status_text?: string;
  agents: Agent[];
  conversations: Conversation[];
  rooms: RoomAggregate[];
  current_agent_id: string | null;
  on_change_app_conversation_draft: (value: string) => void;
  on_open_app_conversation: (initial_prompt?: string) => void;
  on_close_app_conversation: () => void;
  is_app_conversation_open: boolean;
  on_select_agent: (agent_id: string) => void;
  on_stop_app_conversation: () => void;
  on_submit_app_conversation: (prompt: string) => void;
  surface: "launcher" | "app";
}

interface HeroStageProps {
  current_agent_id: string | null;
  decorative_tokens: SpotlightToken[];
  input_disabled?: boolean;
  input_placeholder?: string;
  input_status_text?: string;
  mention_targets: MentionTargetItem[];
  on_enter_home: () => void;
  on_open_app_conversation: (initial_prompt?: string) => void;
  on_close_app_conversation: () => void;
  is_app_conversation_open: boolean;
  on_query_change: (value: string) => void;
  on_select_agent: (agent_id: string) => void;
  on_open_recent_entry: (entry: RecentLauncherEntry) => void;
  on_submit: (submitted_query: string) => boolean;
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

function truncateLauncherChipLabel(label: string, max_chars: number = 8): string {
  const chars = Array.from(label.trim());
  if (chars.length <= max_chars) {
    return label.trim();
  }

  // 中文注释：Hero 推荐项空间很窄，超长名称改为中间省略，
  // 保留首尾辨识信息，避免纯尾部截断导致 DM/Room 名称难以分辨。
  const head_count = Math.max(2, Math.ceil((max_chars - 1) / 2));
  const tail_count = Math.max(2, max_chars - 1 - head_count);
  return `${chars.slice(0, head_count).join("")}…${chars.slice(-tail_count).join("")}`;
}

function isLauncherChipTruncated(label: string, max_chars: number = 6): boolean {
  return Array.from(label.trim()).length > max_chars;
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

function buildLauncherMentionTargets(
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
    const parsed_session_key = parseSessionKey(conversation.session_key);
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

function findLauncherMentionMatch(
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
  input_disabled = false,
  input_placeholder,
  input_status_text,
  mention_targets,
  on_enter_home,
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
    set_mention_match(findLauncherMentionMatch(value, cursor_pos));
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

    // 中文注释：Hero 输入框和 DM/Room composer 一样，提交后先在本地立即清空，
    // 再等待父层异步链路推进，避免受控值回流慢一拍导致残留。
    set_local_query("");
    on_query_change("");
    set_mention_match(null);
  }, [local_query, on_query_change, on_submit]);
  return (
    <div className="relative flex w-full max-w-[1180px] flex-col items-center" onClick={(e) => e.stopPropagation()}>
      <HeroBlobShell
        class_name="z-10 transition-transform duration-500 ease-out"
      >
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
            <h1
              className="mb-2 text-[24px] font-extrabold leading-[1.12] tracking-[-0.05em] text-foreground/96 sm:text-[42px] sm:leading-[1.05]">
              <AnimatedHeroText text={t("launcher.hero_title")} initial_delay_ms={80} stagger_ms={26} />
            </h1>
          </div>
        </div>

        <div className="mt-3 sm:mt-4">
          <FadeSlideIn delay_ms={440} duration_ms={420} y_offset={10}>
            <HeroInputShell class_name="mx-auto w-full max-w-[326px] sm:max-w-[480px]">
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
                <MessageSquare
                  className="h-4.5 w-4.5"
                  style={{ color: "var(--launcher-input-icon)" }}
                />
                <input
                  ref={input_ref}
                  className="flex-1 bg-transparent text-[14px] outline-none shadow-none ring-0 placeholder:text-[color:var(--launcher-input-placeholder)] focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none sm:text-[15px]"
                  style={{
                    color: "var(--launcher-input-text)",
                  }}
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
                  placeholder={input_placeholder || (surface === "app" ? "告诉 Nexus 你要推进什么..." : t("launcher.query_placeholder"))}
                  disabled={surface === "launcher" ? is_query_loading : input_disabled}
                />
                <HeroActionOrbShell class_name="shrink-0" is_active={!is_query_loading}>
                  <button
                    className={cn(
                      "inline-flex h-full w-full items-center justify-center rounded-full transition duration-150 ease-out hover:-translate-y-0.5",
                      ((surface === "launcher" && is_query_loading) || (surface === "app" && input_disabled))
                        && "cursor-not-allowed opacity-50 hover:translate-y-0",
                    )}
                    style={{
                      background: "var(--launcher-submit-background)",
                      boxShadow: "var(--launcher-submit-shadow)",
                      color: "var(--launcher-submit-color)",
                    }}
                    onClick={handle_submit}
                    type="button"
                    disabled={surface === "launcher" ? is_query_loading : input_disabled}
                  >
                    {surface === "app" ? (
                      is_query_loading ? (
                        <RotateCcw className="h-4 w-4" />
                      ) : (
                        <ArrowUp className="h-4 w-4" />
                      )
                    ) : is_query_loading ? (
                      <div className="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-transparent" />
                    ) : (
                      <ArrowUp className="h-4 w-4" />
                    )}
                  </button>
                </HeroActionOrbShell>
              </div>
            </HeroInputShell>
            {surface === "app" && input_status_text ? (
              <p className="mt-2 px-2 text-center text-[11px] text-[color:var(--launcher-handoff-color)]">
                {input_status_text}
              </p>
            ) : null}
          </FadeSlideIn>

          <div className={cn(
            "flex flex-wrap items-center justify-center gap-2",
            surface === "launcher" ? "mt-3 sm:mt-4" : "mt-1 sm:mt-2",
          )}>
            {recent_entries.map((entry, index) => (
              <FadeSlideIn key={entry.key} delay_ms={580 + index * 55} duration_ms={360} y_offset={6} style={{ display: "inline-flex" }}>
                <div className="group relative inline-flex">
                  {isLauncherChipTruncated(entry.label) ? (
                    <div
                      className="pointer-events-none absolute left-1/2 top-full z-20 mt-2 w-max max-w-[220px] -translate-x-1/2 translate-y-1 rounded-2xl px-3 py-2 text-center text-xs font-medium leading-5 opacity-0 shadow-[0_18px_42px_rgba(38,52,76,0.16)] transition duration-200 ease-out group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:translate-y-0 group-focus-within:opacity-100"
                      style={{
                        background: "color-mix(in srgb, rgba(247, 249, 253, 0.96) 88%, rgba(255, 255, 255, 0.72))",
                        boxShadow: "0 18px 42px rgba(38, 52, 76, 0.16), inset 0 0 0 1px rgba(255, 255, 255, 0.58)",
                        color: "rgba(39, 50, 74, 0.88)",
                        backdropFilter: "blur(14px)",
                        WebkitBackdropFilter: "blur(14px)",
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
                    {truncateLauncherChipLabel(entry.label)}
                  </button>
                </div>
              </FadeSlideIn>
            ))}

            <FadeSlideIn delay_ms={580 + recent_entries.length * 55} duration_ms={360} y_offset={6} style={{ display: "inline-flex" }}>
              <button
                className="px-2 text-xs font-medium transition-colors duration-150 ease-out hover:text-[color:var(--launcher-handoff-hover-color)] sm:text-sm"
                style={{ color: "var(--launcher-handoff-color)" }}
                onClick={() => is_app_conversation_open ? on_close_app_conversation() : on_open_app_conversation(query)}
                type="button"
              >
                {is_app_conversation_open ? (
                  <span className="inline-flex items-center gap-1.5">
                    <ArrowLeft className="h-3.5 w-3.5" />
                    {t("launcher.back_to_hub")}
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1.5">
                    {t("launcher.handoff")}
                    <ArrowRight className="h-3.5 w-3.5" />
                  </span>
                )}
              </button>
            </FadeSlideIn>
          </div>
        </div>
      </HeroBlobShell>

      {surface === "launcher" ? (
        <MemoAgentPile
          class_name="hidden min-[400px]:block"
          current_agent_id={current_agent_id}
          on_select_agent={on_select_agent}
          tokens={decorative_tokens}
        />
      ) : null}
    </div>
  );
});

export function LauncherConsole({
  app_conversation_draft,
  app_conversation_loading,
  app_conversation_can_control,
  app_conversation_control_status_text,
  agents,
  conversations,
  rooms,
  current_agent_id,
  on_change_app_conversation_draft,
  on_open_app_conversation,
  on_close_app_conversation,
  is_app_conversation_open,
  on_select_agent,
  on_stop_app_conversation,
  on_submit_app_conversation,
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

  const mention_targets = useMemo(
    () => buildLauncherMentionTargets(agents, rooms, conversations_with_owners),
    [agents, rooms, conversations_with_owners],
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
          const context = await ensureDirectRoom(entry.agent_id);
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

  const handle_submit = useCallback(async (next_query?: string) => {
    const trimmed = (next_query ?? query).trim();
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

  const input_value = surface === "app" ? app_conversation_draft : query;
  const input_loading = surface === "app" ? app_conversation_loading : isQueryLoading;
  const handle_enter_home = useCallback(() => {
    navigate(AppRouteBuilders.home());
  }, [navigate]);

  const handle_input_change = useCallback((value: string) => {
    if (surface === "app") {
      on_change_app_conversation_draft(value);
      return;
    }
    setQuery(value);
  }, [on_change_app_conversation_draft, surface]);

  useEffect(() => {
    if (surface === "app" && query) {
      setQuery("");
    }
  }, [query, surface]);

  const handle_primary_action = useCallback((submitted_input: string) => {
    if (surface === "app") {
      if (!app_conversation_can_control) {
        return false;
      }
      if (app_conversation_loading) {
        on_stop_app_conversation();
        return false;
      }

      const trimmed_input = submitted_input.trim();
      if (!trimmed_input) {
        return false;
      }

      // 中文注释：Launcher Hero 输入框复用 DM/Room 的发送语义，
      // 在异步发送前先立即清空受控草稿，避免回车提交后残留旧文本。
      on_change_app_conversation_draft("");
      void on_submit_app_conversation(trimmed_input);
      return true;
    }

    const trimmed_query = submitted_input.trim();
    if (!trimmed_query || isQueryLoading) {
      return false;
    }

    // 中文注释：Launcher 查询也沿用同样的“先清空、再提交”策略，
    // 保证 Hero 输入框行为与 DM/Room composer 一致。
    setQuery("");
    void handle_submit(trimmed_query);
    return true;
  }, [
    app_conversation_can_control,
    app_conversation_loading,
    handle_submit,
    isQueryLoading,
    on_change_app_conversation_draft,
    on_stop_app_conversation,
    on_submit_app_conversation,
    surface,
  ]);

  return (
    <section className="relative flex h-full min-h-0 flex-1 flex-col overflow-hidden">
      <div className={cn(
        "relative z-30 flex items-center justify-between gap-3 px-3 pt-3 sm:px-7 sm:pt-1",
        surface === "app" && "pointer-events-none absolute inset-x-0 top-0",
      )}
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

      <div className={cn(
        "relative flex min-h-0 flex-1 items-center justify-center px-8",
        surface === "app" ? "pb-0 pt-0" : "pb-8 pt-6",
      )}>
        <HeroStage
          current_agent_id={current_agent_id}
          decorative_tokens={decorative_tokens}
          input_disabled={surface === "app" ? !app_conversation_can_control : false}
          input_status_text={surface === "app" ? app_conversation_control_status_text : undefined}
          mention_targets={mention_targets}
          on_enter_home={handle_enter_home}
          on_open_app_conversation={on_open_app_conversation}
          on_close_app_conversation={on_close_app_conversation}
          is_app_conversation_open={is_app_conversation_open}
          on_query_change={handle_input_change}
          on_select_agent={on_select_agent}
          on_open_recent_entry={handle_open_recent_entry}
          on_submit={handle_primary_action}
          query={input_value}
          recent_entries={recent_entries}
          surface={surface}
          is_query_loading={input_loading}
        />
      </div>
    </section>
  );
}
