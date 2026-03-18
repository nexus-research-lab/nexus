"use client";

import { Bot, FolderKanban, MessageSquare, Plus, Settings, Trash2 } from "lucide-react";

import { Agent } from "@/types/agent";
import { Session } from "@/types/session";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";

interface AgentDirectoryProps {
  agents: Agent[];
  sessions: Session[];
  currentAgentId: string | null;
  onSelectAgent: (agentId: string) => void;
  onCreateAgent: () => void;
  onEditAgent: (agentId: string) => void;
  onDeleteAgent: (agentId: string) => void;
}

export function AgentDirectory({
  agents,
  sessions,
  currentAgentId,
  onSelectAgent,
  onCreateAgent,
  onEditAgent,
  onDeleteAgent,
}: AgentDirectoryProps) {
  const activeAgents = agents.filter((agent) => agent.status === "active").length;
  const totalSessions = sessions.length;
  const activeSessions = sessions.filter((session) => session.is_active !== false).length;

  return (
    <section className="soft-ring radius-shell-xl flex min-h-0 flex-1 flex-col overflow-hidden panel-surface">
      <div className="border-b border-white/55 px-6 py-5">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <span className="text-3xl font-extrabold tracking-[-0.04em] text-foreground">Agent Studio</span>
            <span className="neo-pill rounded-full px-3 py-1.5 text-xs font-semibold">{activeAgents} Agents</span>
            <span className="neo-pill rounded-full px-3 py-1.5 text-xs font-semibold">{totalSessions} Sessions</span>
            <span className="neo-pill rounded-full px-3 py-1.5 text-xs font-semibold">{activeSessions} Active</span>
          </div>

          <button
            className="inline-flex items-center gap-2 rounded-full bg-[linear-gradient(135deg,rgba(255,194,148,0.92),rgba(255,155,86,0.88))] px-5 py-3 text-sm font-bold text-[#8a4409] shadow-[0_18px_34px_rgba(255,157,86,0.28)] transition-transform duration-300 hover:-translate-y-0.5"
            onClick={onCreateAgent}
            type="button"
          >
            <Plus className="h-4 w-4" />
            创建 Agent
          </button>
        </div>
      </div>

      <div className="data-grid soft-scrollbar flex-1 overflow-y-auto px-6 py-6">
        <div className="grid gap-4 xl:grid-cols-2 2xl:grid-cols-3">
          {agents.map((agent) => {
            const agentSessions = sessions.filter((session) => session.agent_id === agent.agent_id);
            const latestSession = agentSessions[0];
            const isActive = currentAgentId === agent.agent_id;
            const model = agent.options.model || "inherit";
            const toolCount = agent.options.allowed_tools?.length ?? 0;

            return (
              <article
                key={agent.agent_id}
                className={cn(
                  "group relative flex min-h-[220px] flex-col overflow-hidden rounded-[34px] p-5 transition-all duration-300",
                  isActive
                    ? "neo-card bg-[linear-gradient(145deg,rgba(225,218,255,0.86),rgba(242,239,234,0.96))] shadow-[0_28px_56px_rgba(134,122,214,0.22)]"
                    : "neo-card hover:-translate-y-1",
                )}
              >
                <div className="pointer-events-none absolute -right-10 -top-10 h-28 w-28 rounded-full bg-white/50 blur-2xl" />
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-2">
                    <div className="neo-pill radius-shell-sm inline-flex h-12 w-12 items-center justify-center text-primary">
                      <Bot className="h-5 w-5" />
                    </div>
                    <div>
                      <h2 className="text-xl font-extrabold tracking-[-0.03em] text-foreground">{agent.name}</h2>
                      <p className="mt-1 text-sm text-muted-foreground">
                        {truncate(agent.workspace_path, 38)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 opacity-0 transition-opacity group-hover:opacity-100">
                    <button
                      aria-label="编辑 Agent 设置"
                      className="neo-pill rounded-2xl p-2 text-muted-foreground transition-colors hover:text-primary focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:ring-offset-1"
                      onClick={() => onEditAgent(agent.agent_id)}
                      type="button"
                    >
                      <Settings className="h-4 w-4" />
                    </button>
                    <button
                      aria-label="删除 Agent"
                      className="neo-pill rounded-2xl p-2 text-muted-foreground transition-colors hover:text-destructive focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:ring-offset-1"
                      onClick={() => onDeleteAgent(agent.agent_id)}
                      type="button"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </div>

                <div className="mt-4 grid grid-cols-2 gap-3">
                  <div className="neo-inset radius-shell-md px-4 py-4">
                    <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">Model</p>
                    <p className="mt-2 text-sm font-bold text-foreground">{model}</p>
                  </div>
                  <div className="neo-inset radius-shell-md px-4 py-4">
                    <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">Tools</p>
                    <p className="mt-2 text-sm font-bold text-foreground">{toolCount}</p>
                  </div>
                </div>

                <div className="mt-4 space-y-2 radius-shell-md neo-card-flat px-4 py-4">
                  <div className="flex items-center gap-2 text-sm text-foreground">
                    <FolderKanban className="h-4 w-4 text-muted-foreground" />
                    <span>{agentSessions.length} 个 Session</span>
                  </div>
                  <div className="flex items-center gap-2 text-sm text-foreground">
                    <MessageSquare className="h-4 w-4 text-muted-foreground" />
                    <span>
                      {latestSession
                        ? `最近活动 ${formatRelativeTime(latestSession.last_activity_at)}`
                        : "尚未创建会话"}
                    </span>
                  </div>
                </div>

                <div className="mt-auto pt-4">
                  <button
                    className={cn(
                      "inline-flex items-center gap-2 rounded-full px-5 py-3 text-sm font-bold transition-all duration-300",
                      isActive
                        ? "bg-[linear-gradient(135deg,rgba(166,255,194,0.92),rgba(102,217,143,0.88))] text-[#18653a] shadow-[0_18px_32px_rgba(102,217,143,0.24)]"
                        : "neo-pill text-foreground hover:-translate-y-0.5 hover:text-primary",
                    )}
                    onClick={() => onSelectAgent(agent.agent_id)}
                    type="button"
                  >
                    进入工作台
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      </div>
    </section>
  );
}
