"use client";

import { useMemo, useState } from "react";
import { ArrowLeft, Check, ChevronDown, MessageSquare, Plus, Search, X } from "lucide-react";

import { ChatInterface } from "@/components/chat/chat-interface";
import { formatRelativeTime } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Session } from "@/types/session";

interface MobileAgentWorkspaceProps {
  currentAgent: Agent;
  currentSession: Session | null;
  currentSessionKey: string | null;
  currentAgentSessions: Session[];
  onBackToDirectory: () => void;
  onNewSession: () => void;
  onSelectSession: (sessionKey: string) => void;
  onLoadingChange: (isLoading: boolean) => void;
  onSessionSnapshotChange: (snapshot: {
    sessionKey: string;
    messageCount: number;
    lastActivityAt: number;
    sessionId: string | null;
  }) => void;
}

export function MobileAgentWorkspace({
  currentAgent,
  currentSession,
  currentSessionKey,
  currentAgentSessions,
  onBackToDirectory,
  onNewSession,
  onSelectSession,
  onLoadingChange,
  onSessionSnapshotChange,
}: MobileAgentWorkspaceProps) {
  const [isSessionSheetOpen, setIsSessionSheetOpen] = useState(false);

  const currentSessionTitle = useMemo(() => {
    if (currentSession?.title?.trim()) {
      return currentSession.title;
    }
    return "新会话";
  }, [currentSession]);

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden bg-background/90">
      <div className="px-2 pb-2 pt-2">
        <div className="panel-surface soft-ring radius-shell-lg flex items-center gap-2 px-2 py-2">
          <button
            className="neo-pill inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-foreground transition hover:text-primary"
            onClick={onBackToDirectory}
            type="button"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>

          <button
            className="neo-inset flex min-w-0 flex-1 items-center gap-3 rounded-[24px] px-3 py-2 text-left transition hover:bg-white/80"
            onClick={() => setIsSessionSheetOpen(true)}
            type="button"
          >
            <div className="neo-pill flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-muted-foreground">
              <Search className="h-4 w-4" />
            </div>

            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold text-foreground">{currentAgent.name}</p>
              <p className="truncate text-[12px] text-muted-foreground">{currentSessionTitle}</p>
            </div>

            <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
          </button>

          <button
            className="neo-pill inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-foreground transition hover:text-primary"
            onClick={() => {
              onNewSession();
              setIsSessionSheetOpen(false);
            }}
            type="button"
          >
            <Plus className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="min-h-0 min-w-0 flex-1">
        <ChatInterface
          agentId={currentAgent.agent_id}
          layout="mobile"
          onLoadingChange={onLoadingChange}
          onSessionSnapshotChange={onSessionSnapshotChange}
          sessionKey={currentSessionKey}
          onNewSession={onNewSession}
        />
      </div>

      {isSessionSheetOpen && (
        <>
          <button
            aria-label="关闭会话列表"
            className="absolute inset-0 z-30 bg-black/20 backdrop-blur-[1px]"
            onClick={() => setIsSessionSheetOpen(false)}
            type="button"
          />

          <div className="absolute inset-x-0 bottom-0 z-40 rounded-t-[28px] border-t border-white/60 bg-[rgba(244,241,236,0.96)] px-4 pb-6 pt-3 shadow-[0_-20px_40px_rgba(0,0,0,0.12)] backdrop-blur-md">
            <div className="mx-auto mb-4 h-1.5 w-12 rounded-full bg-black/10" />

            <div className="mb-4 flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-foreground">切换会话</p>
                <p className="text-xs text-muted-foreground">
                  {currentAgentSessions.length} 个会话
                </p>
              </div>

              <button
                className="inline-flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground transition hover:bg-black/5 hover:text-foreground"
                onClick={() => setIsSessionSheetOpen(false)}
                type="button"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <button
              className="mb-3 flex w-full items-center justify-center gap-2 rounded-2xl bg-[linear-gradient(135deg,rgba(174,163,255,0.18),rgba(255,255,255,0.82))] px-4 py-3 text-sm font-semibold text-foreground shadow-[0_10px_24px_rgba(133,119,255,0.12)]"
              onClick={() => {
                onNewSession();
                setIsSessionSheetOpen(false);
              }}
              type="button"
            >
              <Plus className="h-4 w-4" />
              新建会话
            </button>

            <div className="max-h-[50vh] space-y-2 overflow-y-auto pr-1">
              {currentAgentSessions.map((session) => {
                const isActive = session.session_key === currentSessionKey;
                return (
                  <button
                    key={session.session_key}
                    className="flex w-full items-start gap-3 rounded-2xl border border-white/60 bg-white/50 px-3 py-3 text-left transition hover:bg-white/70"
                    onClick={() => {
                      onSelectSession(session.session_key);
                      setIsSessionSheetOpen(false);
                    }}
                    type="button"
                  >
                    <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[rgba(174,163,255,0.12)] text-primary">
                      {isActive ? <Check className="h-4 w-4" /> : <MessageSquare className="h-4 w-4" />}
                    </div>

                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-foreground">
                        {session.title?.trim() || "未命名会话"}
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {formatRelativeTime(session.last_activity_at)} · {session.message_count ?? 0} 条
                      </p>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        </>
      )}
    </section>
  );
}
