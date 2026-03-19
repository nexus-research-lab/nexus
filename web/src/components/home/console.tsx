"use client";

import { useMemo, useState } from "react";
import {
  ArrowUp,
  ChevronRight,
  MessageSquare,
  Plus,
  Search,
  Settings,
  Trash2,
} from "lucide-react";

import { Agent } from "@/types/agent";
import { Session } from "@/types/session";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";
import { SpotlightToken, AgentPile } from "@/components/home/agent-pile";
import { ANIMATIONS } from "@/components/animations/animations";
import { LottiePlayer } from "@/components/animations/lottiePlayer";

interface AgentDirectoryProps {
  agents: Agent[];
  sessions: Session[];
  currentAgentId: string | null;
  onSelectAgent: (agentId: string) => void;
  onOpenSession: (sessionKey: string, agentId?: string) => void;
  onCreateAgent: () => void;
  onEditAgent: (agentId: string) => void;
  onDeleteAgent: (agentId: string) => void;
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
  sessions: Array<{ session: Session; owner: Agent | null }>,
): SpotlightToken[] {
  const agentTokens: SpotlightToken[] =
    agents.map((agent, index) => ({
      key: `agent-${agent.agent_id}`,
      label: getInitials(agent.name),
      agentId: agent.agent_id,
      kind: "agent" as const,
      swatch: TOKEN_SWATCHES[index % TOKEN_SWATCHES.length],
    }));

  const roomTokens: SpotlightToken[] =
    sessions.slice(0, 8).map(({ session }, index) => ({
      key: `room-${session.session_key}`,
      label: getInitials(session.title || "Room"),
      agentId: session.agent_id ?? null,
      kind: "room" as const,
      swatch: TOKEN_SWATCHES[(agentTokens.length + index) % TOKEN_SWATCHES.length],
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
    ...agentTokens,
    ...roomTokens,
  ];
  fallback.forEach((item, index) => {
    if (source.length < 22) {
      source.push({
        key: `fallback-${item.label}-${index}`,
        label: item.label,
        agentId: null,
        kind: item.kind,
        swatch: TOKEN_SWATCHES[(agentTokens.length + roomTokens.length + index) % TOKEN_SWATCHES.length],
      });
    }
  });

  return source.slice(0, 22);
}

export function Console({
  agents,
  sessions,
  currentAgentId,
  onSelectAgent,
  onOpenSession,
  onCreateAgent,
  onEditAgent,
  onDeleteAgent,
}: AgentDirectoryProps) {
  const [query, setQuery] = useState("");
  const [showContacts, setShowContacts] = useState(false);
  const [showRooms, setShowRooms] = useState(false);
  const [contactsQuery, setContactsQuery] = useState("");
  const [roomsQuery, setRoomsQuery] = useState("");

  const sessionsWithOwners = useMemo(() => {
    return sessions
      .map((session) => ({
        session,
        owner: agents.find((agent) => agent.agent_id === session.agent_id) ?? null,
      }))
      .sort((left, right) => right.session.last_activity_at - left.session.last_activity_at);
  }, [agents, sessions]);

  const recentAgents = useMemo(() => agents.slice(0, 2), [agents]);
  const recentRooms = useMemo(() => sessionsWithOwners.slice(0, 3), [sessionsWithOwners]);
  const decorativeTokens = useMemo(
    () => buildDecorativeTokens(agents, sessionsWithOwners),
    [agents, sessionsWithOwners],
  );

  const filteredContacts = useMemo(() => {
    const keyword = contactsQuery.trim().toLowerCase();
    if (!keyword) {
      return agents;
    }
    return agents.filter((agent) =>
      [agent.name, agent.workspace_path, agent.options.model ?? ""].some((field) =>
        field.toLowerCase().includes(keyword),
      ),
    );
  }, [agents, contactsQuery]);

  const filteredRooms = useMemo(() => {
    const keyword = roomsQuery.trim().toLowerCase();
    if (!keyword) {
      return sessionsWithOwners;
    }
    return sessionsWithOwners.filter(({ session, owner }) =>
      [session.title, owner?.name ?? ""].some((field) =>
        field.toLowerCase().includes(keyword),
      ),
    );
  }, [roomsQuery, sessionsWithOwners]);

  const handleSubmit = () => {
    const trimmed = query.trim();
    if (!trimmed) {
      return;
    }

    const mentionMatch = trimmed.match(/@([^\s#]+)/);
    const roomMatch = trimmed.match(/#([^\s@]+)/);

    if (mentionMatch) {
      const keyword = mentionMatch[1].toLowerCase();
      const matchedAgent = agents.find((agent) => agent.name.toLowerCase().includes(keyword));
      if (matchedAgent) {
        onSelectAgent(matchedAgent.agent_id);
        return;
      }
    }

    if (roomMatch) {
      const keyword = roomMatch[1].toLowerCase();
      const matchedRoom = sessionsWithOwners.find(({ session }) =>
        session.title.toLowerCase().includes(keyword),
      );
      if (matchedRoom) {
        onOpenSession(matchedRoom.session.session_key, matchedRoom.session.agent_id);
        return;
      }
    }

    const roomFirst = sessionsWithOwners.find(({ session }) =>
      session.title.toLowerCase().includes(trimmed.toLowerCase()),
    );
    if (roomFirst) {
      onOpenSession(roomFirst.session.session_key, roomFirst.session.agent_id);
      return;
    }

    const agentFirst = agents.find((agent) =>
      agent.name.toLowerCase().includes(trimmed.toLowerCase()),
    );
    if (agentFirst) {
      onSelectAgent(agentFirst.agent_id);
    }
  };

  return (
    <section className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.36),transparent_38%),radial-gradient(circle_at_bottom,rgba(255,255,255,0.24),transparent_32%)]" />
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <LottiePlayer
          className="absolute left-[4%] top-[-3%] h-22 w-24 opacity-[0.7] z-100"
          src={ANIMATIONS.PRIDE}
        />
        <LottiePlayer
          className="absolute right-[28%] top-[15%] h-24 w-24 opacity-[0.52] z-100"
          src={ANIMATIONS.SPARKLES}
        />
      </div>

      <div className="relative flex items-center justify-between px-7 pt-6">
        <div className="flex items-center gap-3 px-1 py-1">
          <span className="h-7 w-7 rounded-full bg-[#171917]" />
          <span className="text-base font-semibold text-foreground">Nexus</span>
        </div>

        <div className="flex items-center gap-2">
          <button
            className={cn(
              "rounded-full bg-white/62 px-4 py-2 text-sm font-medium text-muted-foreground shadow-[0_3px_10px_rgba(90,102,80,0.03)] transition-all hover:bg-white/80 hover:text-foreground",
              showContacts && "bg-white text-foreground shadow-[0_8px_18px_rgba(90,102,80,0.08)]",
            )}
            onClick={() => {
              setShowContacts((current) => !current);
              setShowRooms(false);
            }}
            type="button"
          >
            Contacts
          </button>
          <button
            className={cn(
              "rounded-full bg-white/62 px-4 py-2 text-sm font-medium text-muted-foreground shadow-[0_3px_10px_rgba(90,102,80,0.03)] transition-all hover:bg-white/80 hover:text-foreground",
              showRooms && "bg-white text-foreground shadow-[0_8px_18px_rgba(90,102,80,0.08)]",
            )}
            onClick={() => {
              setShowRooms((current) => !current);
              setShowContacts(false);
            }}
            type="button"
          >
            Rooms
          </button>
          <button
            className="h-6 w-6 rounded-full bg-[#bff0ca] shadow-[0_0_0_1px_rgba(127,227,168,0.9),0_4px_12px_rgba(127,227,168,0.24)]"
            onClick={onCreateAgent}
            type="button"
            aria-label="创建 Agent"
          />
        </div>
      </div>

      <div className="relative flex min-h-0 flex-1 items-center justify-center px-8 pb-8 pt-6">
        {showContacts && (
          <div className="absolute right-10 top-2 z-20 w-[340px] rounded-[22px] bg-[linear-gradient(180deg,rgba(251,252,248,0.96),rgba(245,247,242,0.94))] p-3 shadow-[0_24px_56px_rgba(90,102,80,0.12),0_2px_8px_rgba(90,102,80,0.04)] backdrop-blur-md">
            <div className="flex items-center gap-2 rounded-[14px] bg-white/78 px-4 py-3 shadow-[inset_0_0_0_1px_rgba(226,230,223,0.72)]">
              <Search className="h-4 w-4 text-muted-foreground" />
              <input
                className="flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
                onChange={(event) => setContactsQuery(event.target.value)}
                placeholder="Search contacts..."
                value={contactsQuery}
              />
            </div>

            <div className="mt-3 space-y-1 px-1">
              {filteredContacts.slice(0, 5).map((agent, index) => (
                <div
                  key={agent.agent_id}
                  className={cn(
                    "flex items-center gap-3 rounded-[14px] px-3 py-2.5 transition-colors hover:bg-white/60",
                    index === 1 && "bg-white/58",
                  )}
                >
                  <button
                    className="flex h-10 w-10 items-center justify-center rounded-full bg-[#f5f7f2]"
                    onClick={() => {
                      setShowContacts(false);
                      onSelectAgent(agent.agent_id);
                    }}
                    type="button"
                  >
                    <span className="text-sm font-semibold text-foreground">
                      {getInitials(agent.name)}
                    </span>
                  </button>

                  <button
                    className="min-w-0 flex-1 text-left"
                    onClick={() => {
                      setShowContacts(false);
                      onSelectAgent(agent.agent_id);
                    }}
                    type="button"
                  >
                    <p className="truncate text-sm font-semibold text-foreground">{agent.name}</p>
                    <div className="mt-1 flex items-center gap-2">
                      <span className="h-2 w-2 rounded-full bg-[#7fe3a8]" />
                      <p className="truncate text-xs text-muted-foreground">
                        {truncate(agent.workspace_path, 22)}
                      </p>
                    </div>
                  </button>

                  <div className="flex items-center gap-1">
                    <button
                      className="rounded-full p-2 text-muted-foreground transition-colors hover:bg-white/80 hover:text-primary"
                      onClick={() => onEditAgent(agent.agent_id)}
                      type="button"
                      aria-label="编辑 Agent 设置"
                    >
                      <Settings className="h-4 w-4" />
                    </button>
                    <button
                      className="rounded-full p-2 text-muted-foreground transition-colors hover:bg-white/80 hover:text-destructive"
                      onClick={() => onDeleteAgent(agent.agent_id)}
                      type="button"
                      aria-label="删除 Agent"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              ))}

              <div className="mt-2 h-px w-full bg-[#e7ebe3]" />

              <button
                className="flex w-full items-center gap-2 rounded-[14px] px-3 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-white/60"
                onClick={onCreateAgent}
                type="button"
              >
                <Plus className="h-4 w-4" />
                New Agent
              </button>
            </div>
          </div>
        )}

        {showRooms && (
          <div className="absolute right-10 top-2 z-20 w-[360px] rounded-[22px] bg-[linear-gradient(180deg,rgba(251,252,248,0.96),rgba(245,247,242,0.94))] p-3 shadow-[0_24px_56px_rgba(90,102,80,0.12),0_2px_8px_rgba(90,102,80,0.04)] backdrop-blur-md">
            <div className="flex items-center gap-2 rounded-[14px] bg-white/78 px-4 py-3 shadow-[inset_0_0_0_1px_rgba(226,230,223,0.72)]">
              <Search className="h-4 w-4 text-muted-foreground" />
              <input
                className="flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
                onChange={(event) => setRoomsQuery(event.target.value)}
                placeholder="Search rooms..."
                value={roomsQuery}
              />
            </div>

            <div className="mt-3 space-y-1 px-1">
              {filteredRooms.slice(0, 4).map(({ session, owner }, index) => (
                <button
                  key={session.session_key}
                  className={cn(
                    "flex w-full items-center justify-between rounded-[14px] px-3 py-2.5 text-left transition-colors hover:bg-white/60",
                    index === 1 && "bg-white/58",
                  )}
                  onClick={() => {
                    setShowRooms(false);
                    onOpenSession(session.session_key, session.agent_id);
                  }}
                  type="button"
                  >
                    <div>
                      <p className="text-sm font-semibold text-foreground">
                        {truncate(session.title || "Untitled Room", 26)}
                      </p>
                      <p className="max-w-[210px] truncate text-xs text-muted-foreground">
                        {(owner?.name ?? "Unknown")} · 最近消息 · {formatRelativeTime(session.last_activity_at)}
                      </p>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="text-[11px] text-muted-foreground">
                        {formatRelativeTime(session.last_activity_at)}
                      </span>
                      {(session.message_count ?? 0) > 0 && (
                        <span className="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-foreground px-1.5 text-[9px] font-bold text-background">
                          {Math.min(session.message_count ?? 0, 9)}
                        </span>
                      )}
                      <ChevronRight className="h-4 w-4 text-muted-foreground" />
                    </div>
                </button>
              ))}

              <div className="mt-2 h-px w-full bg-[#e7ebe3]" />

              <button
                className="flex w-full items-center gap-2 rounded-[14px] px-3 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-white/60"
                onClick={() => {
                  setShowRooms(false);
                  if (recentRooms[0]) {
                    onOpenSession(recentRooms[0].session.session_key, recentRooms[0].session.agent_id);
                  }
                }}
                type="button"
              >
                <Plus className="h-4 w-4" />
                New Room
              </button>
            </div>
          </div>
        )}

        <div className="relative flex w-full max-w-[1180px] flex-col items-center">
          <div className="relative z-10 w-full max-w-[650px] rounded-[32px] bg-[linear-gradient(180deg,rgba(250,251,248,0.68),rgba(246,248,242,0.52))] px-8 py-7 text-center shadow-[0_18px_56px_rgba(107,122,96,0.035),0_2px_10px_rgba(160,166,148,0.025)] backdrop-blur-[10px] sm:px-10 sm:py-9">
            <div className="space-y-3">
              <p className="text-[9px] font-medium uppercase tracking-[0.32em] text-muted-foreground/70">
                Collaboration Hub
              </p>
              <h1 className="text-[38px] font-extrabold tracking-[-0.06em] text-foreground/96 sm:text-[50px] sm:leading-[1.05]">
                和你的 agents 开始协作
              </h1>
              <p className="mx-auto max-w-[42ch] text-sm leading-7 text-muted-foreground/82 sm:text-[15px]">
                用 @Agent 直接对话，或 #Room 进入协作空间。
              </p>
            </div>

            <div className="mt-7 bg-transparent px-1 py-1">
              <div className="flex items-center gap-3 rounded-[22px] bg-[linear-gradient(180deg,rgba(255,255,255,0.82),rgba(250,251,247,0.72))] px-4 py-4 shadow-[0_8px_20px_rgba(90,102,80,0.04),inset_0_0_0_1px_rgba(230,233,227,0.62)]">
                <MessageSquare className="h-4.5 w-4.5 text-muted-foreground/80" />
                <input
                  className="flex-1 bg-transparent text-[15px] text-foreground outline-none placeholder:text-muted-foreground/78"
                  onChange={(event) => setQuery(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      handleSubmit();
                    }
                  }}
                  placeholder="描述意图，@提及 Agent 或 #Room 来启动协作..."
                  value={query}
                />
                <button
                  className="inline-flex h-9 w-9 items-center justify-center rounded-full bg-[#181a18] text-background shadow-[0_8px_16px_rgba(24,26,24,0.14)] transition-transform duration-300 hover:-translate-y-0.5"
                  onClick={handleSubmit}
                  type="button"
                >
                  <ArrowUp className="h-4 w-4" />
                </button>
              </div>

              <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
                {recentAgents.map((agent, index) => (
                  <button
                    key={agent.agent_id}
                    className="inline-flex items-center gap-2 rounded-full bg-white/36 px-3 py-1.5 text-sm font-medium text-foreground/88 shadow-[inset_0_0_0_1px_rgba(234,237,231,0.64)] transition-all duration-300 hover:-translate-y-0.5 hover:bg-white/54"
                    onClick={() => onSelectAgent(agent.agent_id)}
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

                {recentRooms.map(({ session }) => (
                  <button
                    key={session.session_key}
                    className="rounded-full bg-white/28 px-3 py-1.5 text-sm font-medium text-foreground/82 shadow-[inset_0_0_0_1px_rgba(234,237,231,0.56)] transition-all duration-300 hover:-translate-y-0.5 hover:bg-white/46"
                    onClick={() => onOpenSession(session.session_key, session.agent_id)}
                    type="button"
                  >
                    #{truncate(session.title || "Untitled Room", 18)}
                  </button>
                ))}

                <button
                  className="px-2 text-sm font-medium text-muted-foreground/78 transition-colors hover:text-foreground"
                  onClick={() => {
                    setShowContacts(true);
                    setShowRooms(false);
                  }}
                  type="button"
                >
                  See all →
                </button>
              </div>
            </div>
          </div>

          <AgentPile
            currentAgentId={currentAgentId}
            onSelectAgent={onSelectAgent}
            tokens={decorativeTokens}
          />
        </div>
      </div>
    </section>
  );
}
