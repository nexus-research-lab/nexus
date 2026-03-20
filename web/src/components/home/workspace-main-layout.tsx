"use client";

import { RefObject } from "react";

import { ChatInterface } from "@/components/chat/chat-interface";
import { AgentInspector } from "@/components/workspace/agent-inspector";
import { TodoItem } from "@/components/workspace/agent-task-widget";
import { WorkspaceEditorPane } from "@/components/workspace/workspace-editor-pane";
import { WorkspaceSidebar } from "@/components/workspace/workspace-sidebar";
import {
  HOME_AGENT_INSPECTOR_WRAPPER_CLASS,
  HOME_CHAT_PANEL_CLASS,
  HOME_WORKSPACE_MAIN_GAP_CLASS,
} from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";
import { Session } from "@/types/session";

interface WorkspaceMainLayoutProps {
  currentAgent: Agent;
  currentSession: Session | null;
  currentSessionKey: string | null;
  currentAgentSessions: Session[];
  activeWorkspacePath: string | null;
  isEditorOpen: boolean;
  editorWidthPercent: number;
  isResizingEditor: boolean;
  isSessionBusy: boolean;
  currentTodos: TodoItem[];
  sessionCostSummary: SessionCostSummary;
  agentCostSummary: AgentCostSummary;
  workspaceSplitRef: RefObject<HTMLElement | null>;
  onEditAgent: (agentId: string) => void;
  onNewSession: () => void;
  onSelectSession: (sessionKey: string) => void;
  onDeleteSession: (sessionKey: string) => void;
  onOpenWorkspaceFile: (path: string | null) => void;
  onCloseWorkspacePane: () => void;
  onStartEditorResize: () => void;
  onLoadingChange: (isLoading: boolean) => void;
  onTodosChange: (todos: TodoItem[]) => void;
  onSessionSnapshotChange: (snapshot: {
    sessionKey: string;
    messageCount: number;
    lastActivityAt: number;
    sessionId: string | null;
  }) => void;
}

export function WorkspaceMainLayout({
  currentAgent,
  currentSession,
  currentSessionKey,
  currentAgentSessions,
  activeWorkspacePath,
  isEditorOpen,
  editorWidthPercent,
  isResizingEditor,
  isSessionBusy,
  currentTodos,
  sessionCostSummary,
  agentCostSummary,
  workspaceSplitRef,
  onEditAgent,
  onNewSession,
  onSelectSession,
  onDeleteSession,
  onOpenWorkspaceFile,
  onCloseWorkspacePane,
  onStartEditorResize,
  onLoadingChange,
  onTodosChange,
  onSessionSnapshotChange,
}: WorkspaceMainLayoutProps) {
  return (
    <div className={cn("flex min-h-0 min-w-0 flex-1", HOME_WORKSPACE_MAIN_GAP_CLASS)}>
      <div className="hidden lg:flex lg:min-h-0 lg:shrink-0">
        <WorkspaceSidebar
          activeWorkspacePath={activeWorkspacePath}
          agent={currentAgent}
          currentSessionKey={currentSessionKey}
          onCreateSession={onNewSession}
          onDeleteSession={onDeleteSession}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          onSelectSession={onSelectSession}
          sessions={currentAgentSessions}
        />
      </div>

      <section
        ref={workspaceSplitRef}
        className={cn(
          "flex min-h-0 min-w-0 flex-1",
          isEditorOpen ? "gap-0 lg:gap-4" : "gap-0",
          isResizingEditor && "cursor-col-resize select-none",
        )}
      >
        <WorkspaceEditorPane
          agentId={currentAgent.agent_id}
          className="hidden lg:flex"
          isOpen={isEditorOpen}
          onClose={onCloseWorkspacePane}
          onResizeStart={onStartEditorResize}
          path={activeWorkspacePath}
          widthPercent={editorWidthPercent}
        />

        <div className={HOME_CHAT_PANEL_CLASS}>
          <div className="min-h-0 min-w-0 flex-1">
            <ChatInterface
              agentId={currentAgent.agent_id}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              onLoadingChange={onLoadingChange}
              onSessionSnapshotChange={onSessionSnapshotChange}
              onTodosChange={onTodosChange}
              sessionKey={currentSession?.session_key ?? null}
              onNewSession={onNewSession}
            />
          </div>
        </div>
      </section>

      {!isEditorOpen && (
        <div className={HOME_AGENT_INSPECTOR_WRAPPER_CLASS}>
          <AgentInspector
            activeSession={currentSession}
            agent={currentAgent}
            agentCostSummary={agentCostSummary}
            isSessionBusy={isSessionBusy}
            onEditAgent={onEditAgent}
            sessionCostSummary={sessionCostSummary}
            sessions={currentAgentSessions}
            todos={currentTodos}
          />
        </div>
      )}
    </div>
  );
}
