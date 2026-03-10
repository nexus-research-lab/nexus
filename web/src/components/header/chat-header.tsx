"use client";

import { memo } from "react";
import { Activity, PanelTop } from "lucide-react";
import { AgentTaskWidget, TodoItem } from "@/components/todo/agent-task-widget";
import { LoadingOrb } from "@/components/header/loading";

interface ChatHeaderProps {
  sessionKey: string | null;
  isLoading: boolean;
  todos?: TodoItem[];
}


const ChatHeader = memo(({sessionKey, isLoading, todos = []}: ChatHeaderProps) => {
  const activeTask = todos.find(t => t.status === "in_progress");

  return (
    <div className="h-14 border-b border-border/80 flex items-center px-6 justify-between bg-white/70 backdrop-blur-sm z-10">
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <PanelTop size={14}/>
        <span>SESSION WORKSPACE</span>
        <span className="text-border">/</span>
        <span className="text-accent">
          {sessionKey ? `SESSION: ${sessionKey}` : "NEW_SESSION"}
        </span>
        {activeTask && (
          <>
            <span className="text-border">/</span>
            <span className="text-primary flex items-center gap-2">
              <LoadingOrb/>
              <span className="truncate max-w-md">
                {activeTask.activeForm ? activeTask.activeForm : activeTask.content}
              </span>
            </span>
          </>
        )}
      </div>

      <div className="flex items-center gap-3">
        {/* Loading Indicator */}
        <div className="flex items-center gap-2 rounded-full border border-border/80 bg-white/80 px-3 py-2">
          <Activity className="h-3.5 w-3.5 text-muted-foreground" />
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

        {/* Agent Task Widget */}
        <AgentTaskWidget todos={todos}/>
      </div>
    </div>
  );
});

ChatHeader.displayName = "ChatHeader";

export default ChatHeader;
