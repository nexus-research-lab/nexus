"use client";

import { memo } from "react";
import { Activity, PanelTop } from "lucide-react";

interface ChatHeaderProps {
  sessionKey: string | null;
  isLoading: boolean;
}


const ChatHeader = memo(({sessionKey, isLoading}: ChatHeaderProps) => {
  return (
    <div className="z-10 flex h-16 items-center justify-between border-b border-white/55 bg-transparent px-6">
      <div className="flex items-center gap-3 py-2 text-xs text-muted-foreground">
        <div className="neo-pill flex h-10 w-10 items-center justify-center rounded-full">
          <PanelTop size={15}/>
        </div>
        <div>
          <div className="text-[10px] font-semibold uppercase tracking-[0.24em] text-muted-foreground">Session</div>
          <div className="mt-1 text-sm font-semibold text-foreground">
            {sessionKey ? sessionKey : "新会话"}
          </div>
        </div>
      </div>

      <div className="flex items-center gap-3">
        <div className="neo-pill flex items-center gap-2 rounded-full px-4 py-2">
          <Activity className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            {isLoading ? "Running" : "Ready"}
          </span>
          <span className="text-accent">
            ●
          </span>
          {isLoading ? (
            <div className="flex gap-1">
              <div className="w-2 h-2 rounded-full bg-primary animate-[pulse_1s_ease-in-out_infinite]"/>
              <div className="w-2 h-2 rounded-full bg-primary animate-[pulse_1s_ease-in-out_0.2s_infinite]"/>
              <div className="w-2 h-2 rounded-full bg-accent animate-[pulse_1s_ease-in-out_0.4s_infinite]"/>
            </div>
          ) : (
            <>
              <div className="w-2 h-2 rounded-full bg-primary/20"/>
              <div className="w-2 h-2 rounded-full bg-primary/40"/>
              <div className="w-2 h-2 rounded-full bg-accent"/>
            </>
          )}
        </div>
      </div>
    </div>
  );
});

ChatHeader.displayName = "ChatHeader";

export default ChatHeader;
