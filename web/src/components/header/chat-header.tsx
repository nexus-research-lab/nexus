"use client";

import { memo } from "react";
import { Terminal } from "lucide-react";
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
    <div
      className="h-12 border-b border-primary/20 flex items-center px-6 justify-between bg-background/80 backdrop-blur-sm z-10">
      <div className="flex items-center gap-2 text-xs text-primary/60">
        <Terminal size={14}/>
        <span>TERMINAL_ID: CA-9000</span>
        <span className="text-primary/20">|</span>
        <span className="text-accent">
          {sessionKey ? `SESSION: ${sessionKey}` : "NEW_SESSION"}
        </span>
        {activeTask && (
          <>
            <span className="text-primary/20">|</span>
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
        <div className="flex gap-1">
          {isLoading ? (
            <div className="flex gap-1">
              <div className="w-2 h-2 bg-primary animate-[pulse_1s_ease-in-out_infinite]"/>
              <div className="w-2 h-2 bg-primary animate-[pulse_1s_ease-in-out_0.2s_infinite]"/>
              <div className="w-2 h-2 bg-accent animate-[pulse_1s_ease-in-out_0.4s_infinite]"/>
            </div>
          ) : (
            <>
              <div className="w-2 h-2 bg-primary/20"/>
              <div className="w-2 h-2 bg-primary/40"/>
              <div className="w-2 h-2 bg-accent"/>
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