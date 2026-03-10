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
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-[20px] panel-surface">
      <div className="border-b border-border/80 px-5 py-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <span className="font-medium text-foreground">Agent 管理</span>
            <span className="rounded-full border border-border/80 bg-secondary px-3 py-1">{activeAgents} Agents</span>
            <span className="rounded-full border border-border/80 bg-secondary px-3 py-1">{totalSessions} Sessions</span>
            <span className="rounded-full border border-border/80 bg-secondary px-3 py-1">{activeSessions} Active</span>
          </div>

          <button
            className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-sm"
            onClick={onCreateAgent}
            type="button"
          >
            <Plus className="h-4 w-4" />
            创建 Agent
          </button>
        </div>
      </div>

      <div className="data-grid soft-scrollbar flex-1 overflow-y-auto px-5 py-5">
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
                  "group flex min-h-[200px] flex-col rounded-[20px] border bg-card p-4 shadow-sm transition-all",
                  isActive
                    ? "border-primary/30 shadow-[0_18px_48px_rgba(29,95,145,0.16)]"
                    : "border-border/80 hover:-translate-y-1 hover:border-primary/20 hover:shadow-[0_18px_48px_rgba(20,33,43,0.08)]",
                )}
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-2">
                    <div className="inline-flex h-10 w-10 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                      <Bot className="h-5 w-5" />
                    </div>
                    <div>
                      <h2 className="text-base font-semibold text-foreground">{agent.name}</h2>
                      <p className="mt-1 text-sm text-muted-foreground">
                        {truncate(agent.workspace_path, 38)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 opacity-0 transition-opacity group-hover:opacity-100">
                    <button
                      aria-label="编辑 Agent 设置"
                      className="rounded-xl border border-border/80 p-2 text-muted-foreground transition-colors hover:text-primary focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:ring-offset-1"
                      onClick={() => onEditAgent(agent.agent_id)}
                      type="button"
                    >
                      <Settings className="h-4 w-4" />
                    </button>
                    <button
                      aria-label="删除 Agent"
                      className="rounded-xl border border-border/80 p-2 text-muted-foreground transition-colors hover:border-destructive/20 hover:text-destructive focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:ring-offset-1"
                      onClick={() => onDeleteAgent(agent.agent_id)}
                      type="button"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </div>

                <div className="mt-4 grid grid-cols-2 gap-3">
                  <div className="rounded-xl bg-muted/70 px-3 py-3">
                    <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">Model</p>
                    <p className="mt-2 text-sm font-medium text-foreground">{model}</p>
                  </div>
                  <div className="rounded-xl bg-muted/70 px-3 py-3">
                    <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">Tools</p>
                    <p className="mt-2 text-sm font-medium text-foreground">{toolCount}</p>
                  </div>
                </div>

                <div className="mt-4 space-y-2 rounded-xl bg-secondary/90 px-3 py-3">
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
                    className="inline-flex items-center gap-2 rounded-xl border border-primary/20 bg-primary/8 px-4 py-2.5 text-sm font-medium text-primary transition-colors hover:bg-primary hover:text-primary-foreground"
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
