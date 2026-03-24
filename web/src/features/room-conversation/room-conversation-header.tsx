"use client";

import { memo } from "react";
import { Activity, PanelTop } from "lucide-react";

interface RoomConversationHeaderProps {
  currentAgentName: string | null;
  currentConversationId: string | null;
  currentConversationTitle: string | null;
  isLoading: boolean;
}

function getInitials(name: string | null): string {
  if (!name) {
    return "AG";
  }

  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return "AG";
  }
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return `${parts[0][0] ?? ""}${parts[1][0] ?? ""}`.toUpperCase();
}

const RoomConversationHeaderView = memo(({
  currentAgentName,
  currentConversationId,
  currentConversationTitle,
  isLoading,
}: RoomConversationHeaderProps) => {
  return (
    <div className="z-10 flex min-w-0 items-center justify-between overflow-hidden border-b workspace-divider bg-transparent px-8 py-5">
      <div className="flex w-0 min-w-0 flex-1 items-center gap-2 py-1 text-xs text-muted-foreground sm:gap-3">
        <div className="workspace-chip flex h-10 w-10 shrink-0 items-center justify-center rounded-full sm:h-11 sm:w-11">
          <PanelTop size={14} className="text-slate-800/72" />
        </div>
        <div className="min-w-0 flex-1 overflow-hidden">
          <p className="truncate text-[11px] font-semibold uppercase tracking-[0.24em] text-slate-700/48">
            Active Room
          </p>
          <div className="mt-1 truncate text-[32px] font-black tracking-[-0.04em] text-slate-950/90">
            {currentConversationTitle?.trim() || (currentConversationId ? currentConversationId.split(":").at(-1) : "新会话")}
          </div>
          <p className="mt-1 truncate text-[12px] text-slate-700/52">
            围绕这个 room 协作，按上下文调度成员与动作
          </p>
        </div>
      </div>

      <div className="ml-2 flex shrink-0 items-center gap-2 xl:ml-4 xl:gap-3">
        <div className="hidden items-center -space-x-2 xl:flex">
          <div className="workspace-chip flex h-10 w-10 items-center justify-center rounded-full text-[11px] font-bold text-slate-900/82">
            YOU
          </div>
          <div className="workspace-chip flex h-10 w-10 items-center justify-center rounded-full text-[11px] font-bold text-slate-900/82">
            {getInitials(currentAgentName)}
          </div>
        </div>

        <div className="workspace-chip flex h-8 w-8 items-center justify-center rounded-full xl:hidden">
          <span className={isLoading ? "text-emerald-500" : "text-sky-600"}>●</span>
        </div>

        <div className="workspace-chip hidden items-center gap-2 rounded-full px-4 py-2 xl:flex">
          <Activity className="h-3.5 w-3.5 text-slate-700/56" />
          <span className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/56">
            {isLoading ? "Running" : "Ready"}
          </span>
          <span className="text-sky-600/72">●</span>
          {isLoading ? (
            <div className="flex gap-1">
              <div className="h-2 w-2 animate-[pulse_1s_ease-in-out_infinite] rounded-full bg-emerald-400" />
              <div className="h-2 w-2 animate-[pulse_1s_ease-in-out_0.2s_infinite] rounded-full bg-sky-400" />
              <div className="h-2 w-2 animate-[pulse_1s_ease-in-out_0.4s_infinite] rounded-full bg-violet-400" />
            </div>
          ) : (
            <>
              <div className="h-2 w-2 rounded-full bg-emerald-200" />
              <div className="h-2 w-2 rounded-full bg-sky-200" />
              <div className="h-2 w-2 rounded-full bg-violet-200" />
            </>
          )}
        </div>
      </div>
    </div>
  );
});

RoomConversationHeaderView.displayName = "RoomConversationHeaderView";

export function RoomConversationHeader(props: RoomConversationHeaderProps) {
  return <RoomConversationHeaderView {...props} />;
}
