"use client";

import { RefObject } from "react";

import { TodoItem } from "@/components/workspace/agent-task-widget";
import { MobileAgentWorkspace } from "@/components/home/mobile-agent-workspace";
import { WorkspaceMainLayout } from "@/components/home/workspace-main-layout";
import { WorkspaceTopBar } from "@/components/home/workspace-top-bar";
import { useMediaQuery } from "@/hooks/use-media-query";
import { HOME_WORKSPACE_SECTION_GAP_CLASS } from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { Agent } from "@/types/agent";
import { Session } from "@/types/session";
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";

interface AgentWorkspaceShellProps {
  agents: Agent[];
  currentAgent: Agent;
  currentAgentId: string | null;
  recentAgents: Agent[];
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
  onSelectAgent: (agentId: string) => void;
  onOpenCreateAgent: () => void;
  onBackToDirectory: () => void;
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

export function AgentWorkspace({
  agents,
  currentAgent,
  currentAgentId,
  recentAgents,
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
  onSelectAgent,
  onOpenCreateAgent,
  onBackToDirectory,
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
}: AgentWorkspaceShellProps) {
  const isMobile = useMediaQuery("(max-width: 767px)");

  if (isMobile) {
    return (
      <MobileAgentWorkspace
        currentAgent={currentAgent}
        currentSession={currentSession}
        currentSessionKey={currentSessionKey}
        currentAgentSessions={currentAgentSessions}
        onBackToDirectory={onBackToDirectory}
        onLoadingChange={onLoadingChange}
        onNewSession={onNewSession}
        onSelectSession={onSelectSession}
        onSessionSnapshotChange={onSessionSnapshotChange}
      />
    );
  }

  return (
    <section className={cn("flex min-h-0 flex-1 flex-col", HOME_WORKSPACE_SECTION_GAP_CLASS)}>
      <WorkspaceTopBar
        currentAgentName={currentAgent.name}
        agents={agents}
        currentAgentId={currentAgentId}
        recentAgents={recentAgents}
        onSelectAgent={onSelectAgent}
        onOpenDirectory={onBackToDirectory}
        onCreateAgent={onOpenCreateAgent}
      />

      <WorkspaceMainLayout
        activeWorkspacePath={activeWorkspacePath}
        agentCostSummary={agentCostSummary}
        currentAgent={currentAgent}
        currentAgentSessions={currentAgentSessions}
        currentSession={currentSession}
        currentSessionKey={currentSessionKey}
        currentTodos={currentTodos}
        editorWidthPercent={editorWidthPercent}
        isEditorOpen={isEditorOpen}
        isResizingEditor={isResizingEditor}
        isSessionBusy={isSessionBusy}
        onCloseWorkspacePane={onCloseWorkspacePane}
        onDeleteSession={onDeleteSession}
        onEditAgent={onEditAgent}
        onLoadingChange={onLoadingChange}
        onNewSession={onNewSession}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        onSelectSession={onSelectSession}
        onSessionSnapshotChange={onSessionSnapshotChange}
        onStartEditorResize={onStartEditorResize}
        onTodosChange={onTodosChange}
        sessionCostSummary={sessionCostSummary}
        workspaceSplitRef={workspaceSplitRef}
      />
    </section>
  );
}
