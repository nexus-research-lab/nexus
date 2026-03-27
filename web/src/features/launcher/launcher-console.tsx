"use client";

import { memo, useCallback, useMemo, useState } from "react";
import { ArrowUp, MessageSquare } from "lucide-react";

import {
  HeroActionOrbShell,
  HeroBlobShell,
  HeroInputShell,
} from "@/features/launcher/launcher-glass-shell";
import { DebugReferenceOverlay } from "@/features/launcher/launcher-reference-overlay-debug";
import { cn, truncate } from "@/lib/utils";
import { ANIMATIONS } from "@/config/animation-assets";
import { LottiePlayer } from "@/shared/ui/lottie-player";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { ConversationWithOwner, SpotlightToken } from "@/types/launcher";

import { AgentPile } from "./launcher-agent-pile";

interface LauncherConsoleProps {
  agents: Agent[];
  conversations: Conversation[];
  current_agent_id: string | null;
  on_open_app_conversation: (initial_prompt?: string) => void;
  on_close_app_conversation: () => void;
  is_app_conversation_open: boolean;
  on_select_agent: (agent_id: string) => void;
  on_open_conversation: (conversation_id: string, agent_id?: string) => void;
  surface: "launcher" | "app";
}

interface HeroStageProps {
  current_agent_id: string | null;
  decorative_tokens: SpotlightToken[];
  on_open_app_conversation: (initial_prompt?: string) => void;
  on_close_app_conversation: () => void;
  is_app_conversation_open: boolean;
  on_open_conversation: (conversation_id: string, agent_id?: string) => void;
  on_query_change: (value: string) => void;
  on_select_agent: (agent_id: string) => void;
  on_submit: () => void;
  query: string;
  recent_agents: Agent[];
  recent_rooms: ConversationWithOwner[];
  surface: "launcher" | "app";
}


const TOKEN_SWATCHES = [
  {fill: "#5FA052", text: "#FFFFFF", ring: "#8DBA86"},
  {fill: "#E8A838", text: "#FFFFFF", ring: "#F0C56C"},
  {fill: "#4DAA9F", text: "#FFFFFF", ring: "#7CC8BE"},
  {fill: "#A78BFA", text: "#FFFFFF", ring: "#C2B0FF"},
  {fill: "#6C7BDB", text: "#FFFFFF", ring: "#9AA4F2"},
  {fill: "#D4687A", text: "#FFFFFF", ring: "#E597A3"},
  {fill: "#C4A86B", text: "#FFFFFF", ring: "#D7C08D"},
  {fill: "#8B9089", text: "#FFFFFF", ring: "#B6BAB4"},
  {fill: "#E8945A", text: "#FFFFFF", ring: "#F0B186"},
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
    conversations_with_owners.slice(0, 8).map(({conversation}, index) => ({
      key: `room-${conversation.session_key}`,
      label: getInitials(conversation.title || "Room"),
      agent_id: conversation.agent_id ?? null,
      kind: "room" as const,
      swatch: TOKEN_SWATCHES[(agent_tokens.length + index) % TOKEN_SWATCHES.length],
    }));

  const fallback = [
    {label: "SA", kind: "agent" as const},
    {label: "NV", kind: "agent" as const},
    {label: "BO", kind: "agent" as const},
    {label: "DX", kind: "room" as const},
    {label: "WR", kind: "room" as const},
    {label: "QA", kind: "room" as const},
    {label: "SP", kind: "room" as const},
    {label: "AR", kind: "room" as const},
    {label: "NO", kind: "agent" as const},
    {label: "PR", kind: "agent" as const},
    {label: "FL", kind: "agent" as const},
    {label: "PI", kind: "agent" as const},
    {label: "RL", kind: "room" as const},
    {label: "AT", kind: "agent" as const},
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

const HeroStage = memo(function HeroStage({
                                            current_agent_id,
                                            decorative_tokens,
                                            on_open_app_conversation,
                                            on_close_app_conversation,
                                            is_app_conversation_open,
                                            on_open_conversation,
                                            on_query_change,
                                            on_select_agent,
                                            on_submit,
                                            query,
                                            recent_agents,
                                            recent_rooms,
                                            surface,
                                          }: HeroStageProps) {
  return (
    <div className="relative flex w-full max-w-[1180px] flex-col items-center" onClick={(e) => e.stopPropagation()}>
      <DebugReferenceOverlay/>

      <HeroBlobShell
        class_name={cn(
          "z-10 transition-transform duration-500 ease-out lg:origin-left",
          surface === "app" && "lg:translate-x-[-4%] lg:scale-[0.97]",
        )}
      >
        <div className="space-y-3">
          <p className="text-[9px] font-medium uppercase tracking-[0.32em] text-muted-foreground/70">
            Collaboration Hub
          </p>
          <div className="relative inline-block">
            <LottiePlayer
              class_name="pointer-events-none absolute -right-4 -top-5 h-12 w-12 opacity-[0.46] sm:-right-16 sm:-top-14 sm:h-24 sm:w-24"
              inline_style={undefined}
              src={ANIMATIONS.SPARKLES}
            />
            <h1
              className="mb-7 text-[24px] font-extrabold leading-[1.12] tracking-[-0.05em] text-foreground/96 sm:mb-10 sm:text-[42px] sm:leading-[1.05]">
              和你的 agents 开始协作
            </h1>
          </div>
        </div>

        <div className="mt-3 sm:mt-4">
          <HeroInputShell class_name="mx-auto w-full max-w-[326px] sm:max-w-[480px]">
            <div className="flex min-w-0 items-center gap-2.5 sm:gap-3">
              <MessageSquare className="h-4.5 w-4.5 text-black/58"/>
              <input
                className="flex-1 bg-transparent text-[14px] text-white/92 outline-none placeholder:text-black/42 sm:text-[15px]"
                onChange={(event) => on_query_change(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    on_submit();
                  }
                }}
                placeholder="描述意图，@提及 Agent 或 #Room 来启动协作..."
                value={query}
              />
              <button
                className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-white/84 text-slate-900 shadow-[0_10px_20px_rgba(255,255,255,0.16)] transition-transform duration-300 hover:-translate-y-0.5"
                onClick={on_submit}
                type="button"
              >
                <ArrowUp className="h-4 w-4"/>
              </button>
            </div>
          </HeroInputShell>

          <div className="mt-3 flex flex-wrap items-center justify-center gap-2 sm:mt-4">
            {recent_agents.map((agent, index) => (
              <button
                key={agent.agent_id}
                className="inline-flex items-center gap-2 rounded-full bg-white/10 px-3 py-1.5 text-xs font-medium text-white/84 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.12)] transition-all duration-300 hover:-translate-y-0.5 hover:bg-white/18 sm:text-sm"
                onClick={() => on_select_agent(agent.agent_id)}
                type="button"
              >
                <span
                  className="h-4 w-4 rounded-full"
                  style={{
                    backgroundColor: index === 0 ? "#bff0ca" : "#ffd7b8",
                    border: `1px solid ${index === 0 ? "#7fe3a8" : "#e3c6ad"}`,
                  }}
                />
                {agent.name}
              </button>
            ))}

            {recent_rooms.map(({conversation}) => (
              <button
                key={conversation.session_key}
                className="rounded-full bg-white/8 px-3 py-1.5 text-xs font-medium text-white/76 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)] transition-all duration-300 hover:-translate-y-0.5 hover:bg-white/16 sm:text-sm"
                onClick={(e) => {
                  e.stopPropagation();
                  on_open_conversation(conversation.session_key, conversation.agent_id);
                }}
                type="button"
              >
                #{truncate(conversation.title || "Untitled Room", 18)}
              </button>
            ))}

            <button
              className="px-2 text-xs font-medium text-purple-700/52 transition-colors hover:text-purple-700/82 sm:text-sm"
              onClick={() => is_app_conversation_open ? on_close_app_conversation() : on_open_app_conversation(query)}
              type="button"
            >
              交给 Nexus →
            </button>
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
                                  on_open_conversation,
                                  surface,
                                }: LauncherConsoleProps) {
  const [query, setQuery] = useState("");

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

  const recent_agents = useMemo(() => agents.slice(0, 2), [agents]);
  const recent_rooms = useMemo(() => conversations_with_owners.slice(0, 3), [conversations_with_owners]);
  const decorative_tokens = useMemo(
    () => buildDecorativeTokens(agents, conversations_with_owners),
    [agents, conversations_with_owners],
  );

  const handle_submit = useCallback(() => {
    const trimmed = query.trim();
    if (!trimmed) {
      return;
    }

    const mention_match = trimmed.match(/@([^\s#]+)/);
    const room_match = trimmed.match(/#([^\s@]+)/);

    if (mention_match) {
      const keyword = mention_match[1].toLowerCase();
      const matched_agent = agents.find((agent) => agent.name.toLowerCase().includes(keyword));
      if (matched_agent) {
        on_select_agent(matched_agent.agent_id);
        return;
      }
    }

    if (room_match) {
      const keyword = room_match[1].toLowerCase();
      const matched_room = conversations_with_owners.find(({conversation}) =>
        conversation.title.toLowerCase().includes(keyword),
      );
      if (matched_room) {
        on_open_conversation(
          matched_room.conversation.session_key,
          matched_room.conversation.agent_id,
        );
        return;
      }
    }

    const room_first = conversations_with_owners.find(({conversation}) =>
      conversation.title.toLowerCase().includes(trimmed.toLowerCase()),
    );
    if (room_first) {
      on_open_conversation(room_first.conversation.session_key, room_first.conversation.agent_id);
      return;
    }

    const agent_first = agents.find((agent) =>
      agent.name.toLowerCase().includes(trimmed.toLowerCase()),
    );
    if (agent_first) {
      on_select_agent(agent_first.agent_id);
      return;
    }

    on_open_app_conversation(trimmed);
  }, [agents, conversations_with_owners, on_open_app_conversation, on_open_conversation, on_select_agent, query]);

  return (
    <section className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="pointer-events-none absolute inset-0"/>

      <div className="relative z-30 flex items-center justify-between gap-3 px-3 pt-3 sm:px-7 sm:pt-1"
           onClick={(e) => e.stopPropagation()}>
        <div className="relative flex items-center gap-1 px-1 py-1">
          <LottiePlayer
            class_name="pointer-events-none absolute left-10 -top-4 h-12 w-12 opacity-[0.72] sm:left-3 sm:-top-12 sm:h-24 sm:w-24"
            inline_style={undefined}
            src={ANIMATIONS.BOM}
          />
          <img alt="" className="h-9 w-9 sm:h-10 sm:w-10" src="/logo.webp"/>
          <span className="text-sm font-semibold text-foreground sm:text-base">Nexus</span>
        </div>

        <div className="relative z-40 flex items-center gap-1.5 sm:gap-2">
        </div>
      </div>

      <div className="relative flex min-h-0 flex-1 items-center justify-center px-8 pb-8 pt-6">
        <HeroStage
          current_agent_id={current_agent_id}
          decorative_tokens={decorative_tokens}
          on_open_conversation={on_open_conversation}
          on_open_app_conversation={on_open_app_conversation}
          on_close_app_conversation={on_close_app_conversation}
          is_app_conversation_open={is_app_conversation_open}
          on_query_change={setQuery}
          on_select_agent={on_select_agent}
          on_submit={handle_submit}
          query={query}
          recent_agents={recent_agents}
          recent_rooms={recent_rooms}
          surface={surface}
        />
      </div>
    </section>
  );
}
