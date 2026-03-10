"use client";

import {
  Activity,
  CheckSquare,
  Cpu,
  LoaderCircle,
  ShieldCheck,
} from "lucide-react";

import { Agent } from "@/types/agent";
import { Session } from "@/types/session";
import { formatCost, formatRelativeTime, formatTokens, truncate } from "@/lib/utils";
import { TodoItem } from "@/components/todo/agent-task-widget";
import { LoadingOrb } from "@/components/header/loading";
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";

interface AgentInspectorProps {
  agent: Agent;
  sessions: Session[];
  activeSession: Session | null;
  todos: TodoItem[];
  isSessionBusy: boolean;
  sessionCostSummary: SessionCostSummary;
  agentCostSummary: AgentCostSummary;
  onEditAgent: (agentId: string) => void;
}

export function AgentInspector({
                                 agent,
                                 sessions,
                                 activeSession,
                                 todos,
                                 isSessionBusy,
                                 sessionCostSummary,
                                 agentCostSummary,
                                 onEditAgent,
                               }: AgentInspectorProps) {
  const completedTodoCount = todos.filter((todo) => todo.status === "completed").length;
  const activeTodo = todos.find((todo) => todo.status === "in_progress") ?? null;
  const lastRunDurationMs = sessionCostSummary.last_run_duration_ms ?? null;

  return (
    <aside className="flex min-h-0 w-[292px] flex-col rounded-[20px] panel-surface">
      <div className="flex h-12 items-center justify-between border-b border-border/80 px-4">
        <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          Agent State
        </p>
        <button
          aria-label="打开 Agent 设置"
          className="inline-flex h-7 items-center gap-1.5 rounded-xl border border-border/80 bg-secondary/80 px-3 text-xs font-medium leading-4 text-foreground transition-colors hover:border-primary/20 hover:text-primary focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:ring-offset-1"
          onClick={() => onEditAgent(agent.agent_id)}
          type="button"
        >
          设置
        </button>
      </div>

      <div className="flex-1 overflow-y-auto scrollbar-hide">
        <section className="border-b border-border/80 px-3 py-3">
          <div
            className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            <Activity className="h-3.5 w-3.5"/>
            Runtime
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="rounded-xl bg-secondary/80 px-3 py-2.5">
              <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Session</p>
              <p className="mt-1.5 font-semibold text-foreground">{sessions.length}</p>
            </div>
            <div className="rounded-xl bg-secondary/80 px-3 py-2.5">
              <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Status</p>
              <p className="mt-1.5 font-semibold text-foreground">
                {isSessionBusy ? "Running" : activeSession?.is_active === false ? "Idle" : "Active"}
              </p>
            </div>
          </div>
          <div className="mt-3 space-y-2 text-sm">
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Cost Sessions</span>
              <span className="font-medium text-foreground">
                {agentCostSummary.cost_sessions}
              </span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Model</span>
              <span className="font-medium text-foreground">{agent.options.model || "inherit"}</span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Permission</span>
              <span className="font-medium text-foreground">{agent.options.permission_mode || "default"}</span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Last Active</span>
              <span className="font-medium text-foreground">
                {activeSession ? formatRelativeTime(activeSession.last_activity_at) : "未选择"}
              </span>
            </div>
          </div>
        </section>

        <section className="border-b border-border/80 px-3 py-3">
          <div
            className="mb-3 flex items-center justify-between gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            <div className="flex items-center gap-2">
            <CheckSquare className="h-3.5 w-3.5"/>
            Current Plan
            </div>
            <div className="flex items-center gap-2 rounded-full border border-border/70 bg-secondary/70 px-2.5 py-1 text-[11px] tracking-normal normal-case">
              {todos.length > 0 && <LoadingOrb />}
              <span>{todos.length === 0 ? "0 / 0" : `${completedTodoCount} / ${todos.length}`}</span>
            </div>
          </div>
          {activeTodo && (
            <div
              className="flex items-start gap-2 rounded-xl border border-primary/15 bg-primary/6 px-3 py-2.5 text-sm text-foreground">
              <LoaderCircle className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-primary"/>
              <div className="min-w-0">
                <p className="truncate">{activeTodo.content}</p>
                {activeTodo.activeForm && (
                  <p className="mt-1 text-xs text-muted-foreground">{activeTodo.activeForm}</p>
                )}
              </div>
            </div>
          )}
          {todos.length > 0 && (
            <div className="mt-3 space-y-1.5">
              {todos.map((todo, index) => (
                <div
                  key={`${index}-${todo.content}`}
                  className="flex items-start gap-2 rounded-lg border border-border/50 px-2.5 py-2 text-sm"
                >
                  <span
                    className={
                      todo.status === "completed"
                        ? "text-success"
                        : todo.status === "in_progress"
                          ? "text-primary"
                          : "text-muted-foreground"
                    }
                  >
                    {todo.status === "completed" ? "✓" : todo.status === "in_progress" ? "•" : "○"}
                  </span>
                  <span className="min-w-0 flex-1 break-words text-foreground">{todo.content}</span>
                </div>
              ))}
            </div>
          )}
          {todos.length === 0 && (
            <p className="mt-3 text-sm text-muted-foreground">暂无活跃计划。</p>
          )}
        </section>

        <section className="border-b border-border/80 px-3 py-3">
          <div
            className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            <Cpu className="h-3.5 w-3.5"/>
            Token / Cost
          </div>
          <div className="grid grid-cols-2 gap-1.5">
            <div className="rounded-lg bg-secondary/80 px-2.5 py-1.5">
              <p className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">Session</p>
              <p className="mt-0.5 text-sm font-semibold text-foreground">
                {formatCost(sessionCostSummary.total_cost_usd)}
              </p>
            </div>
            <div className="rounded-lg bg-secondary/80 px-2.5 py-1.5">
              <p className="text-[10px] uppercase tracking-[0.12em] text-muted-foreground">Agent</p>
              <p className="mt-0.5 text-sm font-semibold text-foreground">
                {formatCost(agentCostSummary.total_cost_usd)}
              </p>
            </div>
          </div>
          <div className="mt-2 space-y-1 text-xs">
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Tokens</span>
              <span className="font-medium text-foreground">{formatTokens(sessionCostSummary.total_tokens)}</span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">In / Out</span>
              <span className="font-medium text-foreground">
                {formatTokens(sessionCostSummary.total_input_tokens)} / {formatTokens(sessionCostSummary.total_output_tokens)}
              </span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Cache</span>
              <span className="font-medium text-foreground">
                {formatTokens(sessionCostSummary.total_cache_read_input_tokens)} / {formatTokens(sessionCostSummary.total_cache_creation_input_tokens)}
              </span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Last Run</span>
              <span className="font-medium text-foreground">
                {lastRunDurationMs !== null ? `${(lastRunDurationMs / 1000).toFixed(1)}s` : "-"}
              </span>
            </div>
          </div>
        </section>

        <section className="px-3 py-3">
          <div
            className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            <ShieldCheck className="h-3.5 w-3.5"/>
            Policy / Workspace
          </div>
          <div className="space-y-2 text-sm">
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Skills</span>
              <span className="font-medium text-foreground">{agent.options.skills_enabled ? "On" : "Off"}</span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Allowed Tools</span>
              <span className="font-medium text-foreground">{agent.options.allowed_tools?.length ?? 0}</span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Current Session</span>
              <span className="font-medium text-foreground">
                {sessionCostSummary.session_id || activeSession?.session_id || "未选择"}
              </span>
            </div>
          </div>
          <p className="mt-3 text-xs text-muted-foreground">{truncate(agent.workspace_path, 34)}</p>
        </section>
      </div>
    </aside>
  );
}
