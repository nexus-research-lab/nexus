"use client";

import { Bot, FileStack, KeyRound, Settings2, Workflow } from "lucide-react";

import { Agent } from "@/types/agent";
import { Session } from "@/types/session";
import { truncate } from "@/lib/utils";

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
  const allowedTools = agent.options.allowed_tools ?? [];
  const permissionMode = agent.options.permission_mode || "default";

  return (
    <aside className="flex min-h-0 w-[320px] flex-col gap-4">
      <section className="rounded-[24px] panel-surface px-5 py-5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
              Agent Space
            </p>
            <h2 className="mt-2 text-xl font-semibold text-foreground">{agent.name}</h2>
          </div>
          <button
            className="rounded-2xl border border-border/80 bg-white px-4 py-2 text-sm font-medium text-foreground transition-colors hover:border-primary/20 hover:text-primary"
            onClick={() => onEditAgent(agent.agent_id)}
            type="button"
          >
            编辑
          </button>
        </div>

        <div className="mt-5 grid gap-3">
          <div className="rounded-2xl bg-muted/70 px-4 py-3">
            <div className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-muted-foreground">
              <Workflow className="h-3.5 w-3.5" />
              Sessions
            </div>
            <p className="mt-2 text-sm font-medium text-foreground">{sessions.length} 个空间实例</p>
          </div>

          <div className="rounded-2xl bg-muted/70 px-4 py-3">
            <div className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-muted-foreground">
              <Bot className="h-3.5 w-3.5" />
              Model
            </div>
            <p className="mt-2 text-sm font-medium text-foreground">{agent.options.model || "inherit"}</p>
          </div>

          <div className="rounded-2xl bg-muted/70 px-4 py-3">
            <div className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-muted-foreground">
              <KeyRound className="h-3.5 w-3.5" />
              Permission
            </div>
            <p className="mt-2 text-sm font-medium text-foreground">{permissionMode}</p>
          </div>
        </div>
      </section>

      <section className="rounded-[24px] panel-surface px-5 py-5">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          <Settings2 className="h-3.5 w-3.5" />
          Runtime Config
        </div>

        <dl className="mt-4 space-y-4 text-sm">
          <div className="space-y-1">
            <dt className="text-muted-foreground">Workspace</dt>
            <dd className="font-medium text-foreground">{truncate(agent.workspace_path, 36)}</dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">Allowed Tools</dt>
            <dd className="font-medium text-foreground">{allowedTools.length > 0 ? allowedTools.join(", ") : "未限制"}</dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">Skills</dt>
            <dd className="font-medium text-foreground">{agent.options.skills_enabled ? "Enabled" : "Disabled"}</dd>
          </div>
        </dl>
      </section>

      <section className="flex-1 rounded-[24px] panel-surface px-5 py-5">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          <FileStack className="h-3.5 w-3.5" />
          Active Context
        </div>

        <div className="mt-4 space-y-3">
          <div className="rounded-2xl bg-secondary/90 px-4 py-4">
            <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">Current Session</p>
            <p className="mt-2 text-sm font-medium text-foreground">
              {activeSession?.title || "尚未选择 Session"}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              {activeSession ? `${activeSession.message_count ?? 0} 条消息` : "进入 Session 后查看运行时间线"}
            </p>
          </div>

          <div className="rounded-2xl border border-dashed border-border/80 bg-white/70 px-4 py-4 text-sm leading-6 text-muted-foreground">
            右侧检查器后续可以继续承载 Permission Queue、Workspace 文件、Memory 摘要和 Tool Trace。这次先把 Agent Space 的结构骨架立住。
          </div>
        </div>
      </section>
    </aside>
  );
}
