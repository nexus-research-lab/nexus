"use client";

import {
  Activity,
  BrainCircuit,
  CheckSquare,
  Cpu,
  LoaderCircle,
  ShieldCheck,
  Waypoints,
} from "lucide-react";

import { Agent } from "@/types/agent";
import { Session } from "@/types/session";
import { formatCost, formatRelativeTime, formatTokens, truncate } from "@/lib/utils";
import { TodoItem } from "@/components/todo/agent-task-widget";
import { SessionTelemetry } from "@/types/telemetry";
import { LoadingOrb } from "@/components/header/loading";

interface AgentInspectorProps {
  agent: Agent;
  sessions: Session[];
  activeSession: Session | null;
  todos: TodoItem[];
  isSessionBusy: boolean;
  telemetry: SessionTelemetry;
  onEditAgent: (agentId: string) => void;
}

export function AgentInspector({
                                 agent,
                                 sessions,
                                 activeSession,
                                 todos,
                                 isSessionBusy,
                                 telemetry,
                                 onEditAgent,
                               }: AgentInspectorProps) {
  const maxTurns = agent.options.max_turns ?? 24;
  const turnUsage = activeSession?.message_count ?? 0;
  const contextRatio = Math.min(turnUsage / Math.max(maxTurns, 1), 1);
  const completedTodoCount = todos.filter((todo) => todo.status === "completed").length;
  const activeTodo = todos.find((todo) => todo.status === "in_progress") ?? null;

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
                {telemetry.pending_permission
                  ? "Awaiting Approval"
                  : isSessionBusy
                    ? "Running"
                    : activeSession?.is_active === false
                      ? "Idle"
                      : "Active"}
              </p>
            </div>
          </div>
          <div className="mt-3 space-y-2 text-sm">
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Approvals</span>
              <span className="font-medium text-foreground">
                {telemetry.pending_permission ? "1 pending" : "0 pending"}
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
            className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            <BrainCircuit className="h-3.5 w-3.5"/>
            Context Capacity
          </div>
          <div
            className="h-2 overflow-hidden rounded-full bg-secondary/80"
            role="progressbar"
            aria-valuenow={Math.round(contextRatio * 100)}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label="上下文使用率"
          >
            <div
              className="h-full rounded-full bg-primary transition-all"
              style={{width: `${Math.max(contextRatio * 100, 8)}%`}}
            />
          </div>
          <div className="mt-3 flex justify-between gap-4 text-sm">
            <span className="text-muted-foreground">Messages / Max Turns</span>
            <span className="font-medium text-foreground">
              {turnUsage} / {maxTurns}
            </span>
          </div>
        </section>

        <section className="border-b border-border/80 px-3 py-3">
          <div
            className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            <Cpu className="h-3.5 w-3.5"/>
            Token / Cost
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="rounded-xl bg-secondary/80 px-3 py-2.5">
              <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Total Tokens</p>
              <p className="mt-1.5 font-semibold text-foreground">{formatTokens(telemetry.usage.total_tokens)}</p>
            </div>
            <div className="rounded-xl bg-secondary/80 px-3 py-2.5">
              <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Total Cost</p>
              <p className="mt-1.5 font-semibold text-foreground">
                {formatCost(telemetry.usage.total_cost_usd)}
              </p>
            </div>
            <div className="rounded-xl bg-secondary/80 px-3 py-2.5">
              <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Input / Output</p>
              <p className="mt-1.5 font-semibold text-foreground">
                {formatTokens(telemetry.usage.input_tokens)} / {formatTokens(telemetry.usage.output_tokens)}
              </p>
            </div>
            <div className="rounded-xl bg-secondary/80 px-3 py-2.5">
              <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Last Run</p>
              <p className="mt-1.5 font-semibold text-foreground">
                {telemetry.usage.latest_duration_ms !== null
                  ? `${(telemetry.usage.latest_duration_ms / 1000).toFixed(1)}s`
                  : "-"}
              </p>
            </div>
          </div>
        </section>

        <section className="border-b border-border/80 px-3 py-3">
          <div
            className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            <LoaderCircle className="h-3.5 w-3.5"/>
            Approval Queue
          </div>
          {telemetry.pending_permission ? (
            <div className="rounded-xl bg-secondary/80 px-3 py-2.5 text-sm">
              <div className="flex items-center justify-between gap-3">
                <span className="truncate font-medium text-foreground">
                  {telemetry.pending_permission.tool_name}
                </span>
                <span className="rounded-full border border-warning/20 bg-warning/10 px-2 py-1 text-[11px] text-warning">
                  Pending
                </span>
              </div>
              <p className="mt-2 text-xs text-muted-foreground">
                request_id: {telemetry.pending_permission.request_id}
              </p>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">当前没有等待处理的权限请求。</p>
          )}
        </section>

        <section className="border-b border-border/80 px-3 py-3">
          <div
            className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            <Waypoints className="h-3.5 w-3.5"/>
            Trace Telemetry
          </div>
          {telemetry.tool_calls.length > 0 ? (
            <div className="space-y-1.5">
              {telemetry.tool_calls.slice(-6).reverse().map((toolCall) => (
                <div key={toolCall.id} className="rounded-xl bg-secondary/80 px-3 py-2.5 text-sm">
                  <div className="flex items-center justify-between gap-3">
                    <span className="truncate font-medium text-foreground">{toolCall.tool_name}</span>
                    <span className="text-xs text-muted-foreground">{toolCall.status}</span>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {new Date(toolCall.start_time).toLocaleTimeString("zh-CN", {
                      hour: "2-digit",
                      minute: "2-digit",
                      second: "2-digit",
                    })}
                  </p>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">当前没有可展示的 trace 事件。</p>
          )}
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
                {activeSession?.title || "未选择"}
              </span>
            </div>
          </div>
          <p className="mt-3 text-xs text-muted-foreground">{truncate(agent.workspace_path, 34)}</p>
        </section>
      </div>
    </aside>
  );
}
