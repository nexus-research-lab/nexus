"use client";

import {
  Activity,
  BrainCircuit,
  CheckSquare,
  Cpu,
  ShieldCheck,
  Waypoints,
} from "lucide-react";

import { Agent } from "@/types/agent";
import { Session } from "@/types/session";
import { formatRelativeTime, truncate } from "@/lib/utils";

interface AgentInspectorProps {
  agent: Agent;
  sessions: Session[];
  activeSession: Session | null;
  onEditAgent: (agentId: string) => void;
}

export function AgentInspector({
  agent,
  sessions,
  activeSession,
  onEditAgent,
}: AgentInspectorProps) {
  const estimatedTokens = Math.max((activeSession?.message_count ?? 0) * 320, 0);
  const maxTurns = agent.options.max_turns ?? 24;
  const turnUsage = activeSession?.message_count ?? 0;
  const contextRatio = Math.min(turnUsage / Math.max(maxTurns, 1), 1);

  return (
    <aside className="flex min-h-0 w-[292px] flex-col rounded-[20px] panel-surface">
      <div className="border-b border-border/80 px-4 py-3">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
              Agent State
            </p>
            <h2 className="mt-1 truncate text-sm font-semibold text-foreground">{agent.name}</h2>
          </div>
          <button
            className="rounded-xl border border-border/80 bg-secondary/80 px-3 py-2 text-sm font-medium text-foreground transition-colors hover:border-primary/20 hover:text-primary"
            onClick={() => onEditAgent(agent.agent_id)}
            type="button"
          >
            设置
          </button>
        </div>
      </div>

      <div className="soft-scrollbar flex-1 overflow-y-auto px-4 py-4">
        <div className="space-y-3">
          <section className="rounded-2xl bg-muted/70 px-4 py-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              <Activity className="h-3.5 w-3.5" />
              Runtime
            </div>
            <div className="mt-3 grid grid-cols-2 gap-2">
              <div className="rounded-xl bg-background px-3 py-3">
                <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Session</p>
                <p className="mt-2 text-sm font-semibold text-foreground">{sessions.length}</p>
              </div>
              <div className="rounded-xl bg-background px-3 py-3">
                <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Status</p>
                <p className="mt-2 text-sm font-semibold text-foreground">
                  {activeSession?.is_active === false ? "Idle" : "Active"}
                </p>
              </div>
            </div>
            <div className="mt-3 space-y-2 text-sm">
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

          <section className="rounded-2xl bg-muted/70 px-4 py-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              <BrainCircuit className="h-3.5 w-3.5" />
              Context Capacity
            </div>
            <div className="mt-3">
              <div className="h-2 overflow-hidden rounded-full bg-background">
                <div
                  className="h-full rounded-full bg-primary transition-all"
                  style={{ width: `${Math.max(contextRatio * 100, 8)}%` }}
                />
              </div>
              <div className="mt-3 flex justify-between gap-4 text-sm">
                <span className="text-muted-foreground">Messages / Max Turns</span>
                <span className="font-medium text-foreground">
                  {turnUsage} / {maxTurns}
                </span>
              </div>
              <p className="mt-2 text-xs text-muted-foreground">
                当前以前端已知消息数近似映射 context pressure，后续可接真实 harness telemetry。
              </p>
            </div>
          </section>

          <section className="rounded-2xl bg-muted/70 px-4 py-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              <Cpu className="h-3.5 w-3.5" />
              Token / Cost
            </div>
            <div className="mt-3 grid grid-cols-2 gap-2">
              <div className="rounded-xl bg-background px-3 py-3">
                <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Estimated</p>
                <p className="mt-2 text-sm font-semibold text-foreground">{estimatedTokens.toLocaleString()}</p>
              </div>
              <div className="rounded-xl bg-background px-3 py-3">
                <p className="text-[11px] uppercase tracking-[0.14em] text-muted-foreground">Thinking</p>
                <p className="mt-2 text-sm font-semibold text-foreground">
                  {agent.options.max_thinking_tokens ?? "-"}
                </p>
              </div>
            </div>
          </section>

          <section className="rounded-2xl bg-muted/70 px-4 py-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              <CheckSquare className="h-3.5 w-3.5" />
              Current Plan
            </div>
            <p className="mt-3 text-sm text-muted-foreground">
              当前前端还没有接入实时 todo / plan telemetry，这里保留为 harness planning 区域。
            </p>
            <div className="mt-3 rounded-xl bg-background px-3 py-3 text-sm text-muted-foreground">
              暂无活跃计划。后续可接入 planner / todo 列表、阻塞项与审批节点。
            </div>
          </section>

          <section className="rounded-2xl bg-muted/70 px-4 py-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              <Waypoints className="h-3.5 w-3.5" />
              Orchestration
            </div>
            <div className="mt-3 space-y-2 text-sm">
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">Subagents</span>
                <span className="font-medium text-foreground">待接入</span>
              </div>
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">Agent Team</span>
                <span className="font-medium text-foreground">待接入</span>
              </div>
            </div>
          </section>

          <section className="rounded-2xl bg-muted/70 px-4 py-4">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              <ShieldCheck className="h-3.5 w-3.5" />
              Policy / Workspace
            </div>
            <div className="mt-3 space-y-2 text-sm">
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
      </div>
    </aside>
  );
}
