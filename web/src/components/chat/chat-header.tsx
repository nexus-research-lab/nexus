"use client";

import { memo } from "react";
import { Activity, PanelTop } from "lucide-react";

interface ChatHeaderProps {
  sessionKey: string | null;
  isLoading: boolean;
}


const ChatHeader = memo(({ sessionKey, isLoading }: ChatHeaderProps) => {
  return (
    <div className="z-10 flex min-w-0 items-center justify-between overflow-hidden border-b border-white/55 bg-transparent px-2 h-14 xl:px-6">
      <div className="flex w-0 min-w-0 flex-1 items-center gap-2 py-2 text-xs text-muted-foreground sm:gap-3">
        <div className="neo-pill flex h-9 w-9 shrink-0 items-center justify-center rounded-full sm:h-10 sm:w-10">
          <PanelTop size={14} />
        </div>
        <div className="min-w-0 flex-1 overflow-hidden">
          <div className="mt-1 uppercase truncate text-xs font-semibold text-muted-foreground">
            Session: {sessionKey ? sessionKey.split(":").at(-1) : "新会话"}
          </div>
        </div>
      </div>

      <div className="ml-2 flex shrink-0 items-center gap-2 xl:ml-4 xl:gap-3">
        <div className="neo-pill flex h-8 w-8 items-center justify-center rounded-full xl:hidden">
          <span className={isLoading ? "text-primary" : "text-accent"}>●</span>
        </div>

        <div className="neo-pill hidden items-center gap-2 rounded-full px-4 py-2 xl:flex">
          <Activity className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            {isLoading ? "Running" : "Ready"}
          </span>
          <span className="text-accent">
            ●
          </span>
          {isLoading ? (
            <div className="flex gap-1">
              <div className="w-2 h-2 rounded-full bg-primary animate-[pulse_1s_ease-in-out_infinite]" />
              <div className="w-2 h-2 rounded-full bg-primary animate-[pulse_1s_ease-in-out_0.2s_infinite]" />
              <div className="w-2 h-2 rounded-full bg-accent animate-[pulse_1s_ease-in-out_0.4s_infinite]" />
            </div>
          ) : (
            <>
              <div className="w-2 h-2 rounded-full bg-primary/20" />
              <div className="w-2 h-2 rounded-full bg-primary/40" />
              <div className="w-2 h-2 rounded-full bg-accent" />
            </>
          )}
        </div>
      </div>
    </div>
  );
});

ChatHeader.displayName = "ChatHeader";

export default ChatHeader;
