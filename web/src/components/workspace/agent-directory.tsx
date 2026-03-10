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
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-[28px] panel-surface">
      <div className="border-b border-border/80 px-8 py-7">
        <div className="flex flex-wrap items-start justify-between gap-6">
          <div className="max-w-2xl space-y-3">
            <p className="text-xs font-semibold uppercase tracking-[0.22em] text-primary">
              Agent Directory
            </p>
            <h1 className="text-3xl font-semibold tracking-tight text-foreground">
              管理你的 Agent 资产，而不是只管理一段对话
            </h1>
            <p className="text-sm leading-6 text-muted-foreground">
              Agent 作为长期配置和工作空间入口，Session 作为运行实例进入二级空间。先选 Agent，再进入它的工作台处理会话、权限和执行记录。
            </p>
          </div>

          <button
            className="inline-flex items-center gap-2 rounded-2xl bg-primary px-5 py-3 text-sm font-medium text-primary-foreground shadow-sm transition-transform hover:-translate-y-0.5"
            onClick={onCreateAgent}
            type="button"
          >
            <Plus className="h-4 w-4" />
            创建 Agent
          </button>
        </div>

        <div className="mt-6 grid gap-3 md:grid-cols-3">
          <div className="rounded-2xl panel-subtle px-5 py-4">
            <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Active Agents
            </p>
            <p className="mt-3 text-3xl font-semibold text-foreground">{activeAgents}</p>
          </div>
          <div className="rounded-2xl panel-subtle px-5 py-4">
            <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Sessions
            </p>
            <p className="mt-3 text-3xl font-semibold text-foreground">{totalSessions}</p>
          </div>
          <div className="rounded-2xl panel-subtle px-5 py-4">
            <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Running Spaces
            </p>
            <p className="mt-3 text-3xl font-semibold text-foreground">{activeSessions}</p>
          </div>
        </div>
      </div>

      <div className="data-grid soft-scrollbar flex-1 overflow-y-auto px-8 py-8">
        <div className="grid gap-5 xl:grid-cols-2 2xl:grid-cols-3">
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
                  "group flex min-h-[240px] flex-col rounded-[24px] border bg-white/90 p-5 shadow-sm transition-all",
                  isActive
                    ? "border-primary/30 shadow-[0_18px_48px_rgba(29,95,145,0.16)]"
                    : "border-border/80 hover:-translate-y-1 hover:border-primary/20 hover:shadow-[0_18px_48px_rgba(20,33,43,0.08)]",
                )}
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-3">
                    <div className="inline-flex h-11 w-11 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                      <Bot className="h-5 w-5" />
                    </div>
                    <div>
                      <h2 className="text-lg font-semibold text-foreground">{agent.name}</h2>
                      <p className="mt-1 text-sm text-muted-foreground">
                        {truncate(agent.workspace_path, 38)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 opacity-0 transition-opacity group-hover:opacity-100">
                    <button
                      className="rounded-xl border border-border/80 p-2 text-muted-foreground transition-colors hover:text-primary"
                      onClick={() => onEditAgent(agent.agent_id)}
                      type="button"
                    >
                      <Settings className="h-4 w-4" />
                    </button>
                    <button
                      className="rounded-xl border border-border/80 p-2 text-muted-foreground transition-colors hover:border-destructive/20 hover:text-destructive"
                      onClick={() => onDeleteAgent(agent.agent_id)}
                      type="button"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </div>

                <div className="mt-6 grid grid-cols-2 gap-3">
                  <div className="rounded-2xl bg-muted/70 px-4 py-3">
                    <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">Model</p>
                    <p className="mt-2 text-sm font-medium text-foreground">{model}</p>
                  </div>
                  <div className="rounded-2xl bg-muted/70 px-4 py-3">
                    <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">Tools</p>
                    <p className="mt-2 text-sm font-medium text-foreground">{toolCount}</p>
                  </div>
                </div>

                <div className="mt-5 space-y-2 rounded-2xl bg-secondary/90 px-4 py-4">
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

                <div className="mt-auto pt-5">
                  <button
                    className="inline-flex items-center gap-2 rounded-2xl border border-primary/20 bg-primary/8 px-4 py-3 text-sm font-medium text-primary transition-colors hover:bg-primary hover:text-primary-foreground"
                    onClick={() => onSelectAgent(agent.agent_id)}
                    type="button"
                  >
                    进入 Agent Space
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
