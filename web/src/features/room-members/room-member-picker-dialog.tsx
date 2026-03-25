"use client";

import { X } from "lucide-react";

import { Agent } from "@/types/agent";
import { cn } from "@/lib/utils";

interface RoomMemberPickerDialogProps {
  agents: Agent[];
  is_open: boolean;
  on_cancel: () => void;
  on_select: (agent_id: string) => void;
}

export function RoomMemberPickerDialog({
  agents,
  is_open,
  on_cancel,
  on_select,
}: RoomMemberPickerDialogProps) {
  if (!is_open) {
    return null;
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
    >
      <div className="soft-ring radius-shell-lg panel-surface w-full max-w-lg p-5">
        <div className="flex items-start justify-between gap-3 pb-3">
          <div>
            <h3 className="text-base font-semibold text-foreground">添加成员</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              选择一个已有成员加入当前 room。
            </p>
          </div>
          <button
            aria-label="关闭"
            className="neo-pill radius-shell-sm p-1 text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={on_cancel}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {agents.length === 0 ? (
          <div className="radius-shell-md neo-inset px-4 py-4 text-sm text-muted-foreground">
            当前没有可添加的成员。
          </div>
        ) : (
          <div className="max-h-[360px] space-y-2 overflow-y-auto pr-1">
            {agents.map((agent) => (
              <button
                key={agent.agent_id}
                className={cn(
                  "workspace-card flex w-full items-center gap-3 rounded-[20px] px-4 py-3 text-left transition-all duration-300 hover:-translate-y-0.5",
                )}
                onClick={() => on_select(agent.agent_id)}
                type="button"
              >
                <div className="workspace-chip flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-[11px] font-bold text-slate-900/82">
                  {agent.name.slice(0, 2).toUpperCase()}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold text-slate-900/88">
                    {agent.name}
                  </p>
                  <p className="truncate text-[11px] text-slate-700/52">
                    选择后会加入当前 room 的所有对话
                  </p>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
