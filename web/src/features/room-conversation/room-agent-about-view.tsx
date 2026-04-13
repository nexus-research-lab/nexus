"use client";

import { Bot, Shield } from "lucide-react";

import { WorkspaceSurfaceView } from "@/shared/ui/workspace/workspace-surface-view";
import { Agent } from "@/types/agent";

interface RoomAgentAboutViewProps {
  agent: Agent;
}

export function RoomAgentAboutView({ agent }: RoomAgentAboutViewProps) {
  return (
    <WorkspaceSurfaceView eyebrow="About" title={agent.name}>
      <div className="py-5">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-[14px] border border-(--divider-subtle-color) text-(--icon-strong)">
            <Bot className="h-5 w-5" />
          </div>
          <div>
            <p className="text-base font-semibold text-(--text-strong)">{agent.name}</p>
            <p className="text-[13px] text-(--text-muted)">单成员协作对象</p>
          </div>
        </div>

        <div className="mt-4 grid gap-4 border-t border-(--divider-subtle-color) pt-4 sm:grid-cols-2">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
              Model
            </p>
            <p className="mt-1.5 text-[13px] font-semibold text-(--text-strong)">
              {agent.options.model || "inherit"}
            </p>
          </div>
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
              Permission
            </p>
            <p className="mt-1.5 inline-flex items-center gap-2 text-[13px] font-semibold text-(--text-strong)">
              <Shield className="h-4 w-4 text-(--icon-default)" />
              {agent.options.permission_mode || "default"}
            </p>
          </div>
        </div>
      </div>
    </WorkspaceSurfaceView>
  );
}
