"use client";

import { Clock3, MessageSquarePlus, Trash2 } from "lucide-react";

import { Session } from "@/types/session";
import { cn, formatRelativeTime, truncate } from "@/lib/utils";

interface SessionRailProps {
  sessions: Session[];
  currentSessionKey: string | null;
  onSelectSession: (sessionKey: string) => void;
  onCreateSession: () => void;
  onDeleteSession: (sessionKey: string) => void;
}

export function SessionRail({
  sessions,
  currentSessionKey,
  onSelectSession,
  onCreateSession,
  onDeleteSession,
}: SessionRailProps) {
  return (
    <aside className="flex min-h-0 w-[288px] flex-col rounded-[24px] panel-surface">
      <div className="border-b border-border/80 px-5 py-5">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
              Sessions
            </p>
            <p className="mt-2 text-lg font-semibold text-foreground">{sessions.length} 个会话</p>
          </div>
          <button
            className="inline-flex items-center gap-2 rounded-2xl bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground"
            onClick={onCreateSession}
            type="button"
          >
            <MessageSquarePlus className="h-4 w-4" />
            新会话
          </button>
        </div>
      </div>

      <div className="soft-scrollbar flex-1 overflow-y-auto px-3 py-3">
        {sessions.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-border/80 bg-secondary/80 px-4 py-5 text-sm leading-6 text-muted-foreground">
            当前 Agent 还没有 Session。先创建一个会话，再进入执行时间线。
          </div>
        ) : (
          <div className="space-y-2">
            {sessions.map((session) => {
              const isActive = session.session_key === currentSessionKey;
              return (
                <button
                  key={session.session_key}
                  className={cn(
                    "group w-full rounded-2xl border px-4 py-4 text-left transition-all",
                    isActive
                      ? "border-primary/30 bg-primary/8 shadow-sm"
                      : "border-transparent bg-white/70 hover:border-border/90 hover:bg-white",
                  )}
                  onClick={() => onSelectSession(session.session_key)}
                  type="button"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-foreground">
                        {truncate(session.title || "Untitled Session", 24)}
                      </p>
                      <div className="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
                        <Clock3 className="h-3.5 w-3.5" />
                        <span>{formatRelativeTime(session.last_activity_at)}</span>
                      </div>
                      <p className="mt-2 text-xs text-muted-foreground">
                        {session.message_count ?? 0} 条消息
                      </p>
                    </div>

                    <span
                      className={cn(
                        "rounded-full px-2 py-1 text-[11px] font-medium",
                        session.is_active === false
                          ? "bg-muted text-muted-foreground"
                          : "bg-accent/10 text-accent",
                      )}
                    >
                      {session.is_active === false ? "Idle" : "Active"}
                    </span>
                  </div>

                  <div className="mt-4 flex items-center justify-end opacity-0 transition-opacity group-hover:opacity-100">
                    <button
                      className="rounded-xl border border-border/80 p-2 text-muted-foreground transition-colors hover:border-destructive/20 hover:text-destructive"
                      onClick={(event) => {
                        event.stopPropagation();
                        onDeleteSession(session.session_key);
                      }}
                      type="button"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </aside>
  );
}
