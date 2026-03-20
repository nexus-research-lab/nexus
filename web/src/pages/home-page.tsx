/**
 * 主页面 — Agent Directory + Agent Space
 *
 * [INPUT]: 依赖 SessionStore, AgentStore, Agent Directory/Space 组件
 * [OUTPUT]: 对外提供 B 端控制台首页
 * [POS]: Home 页面，负责编排 Agent 目录、Agent 工作台与对话视图
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { AgentOptions } from "@/components/dialog/agent-options";
import { AgentWorkspace } from "@/components/home/agent-workspace";
import { Console } from "@/components/home/console";
import { HomeLoadingScreen } from "@/components/home/home-loading-screen";
import { useHomePageController } from "@/hooks/use-home-page-controller";
import { HOME_PAGE_PADDING_CLASS } from "@/lib/home-layout";

export function HomePage() {
  const controller = useHomePageController();

  if (!controller.isHydrated) {
    return <HomeLoadingScreen />;
  }

  return (
    <main className="relative flex h-screen w-full overflow-hidden bg-background text-foreground">
      <div className="pointer-events-none absolute left-[5%] top-[8%] h-72 w-72 rounded-full glow-lilac opacity-55" />
      <div className="pointer-events-none absolute bottom-[6%] left-[22%] h-80 w-80 rounded-full bg-white/40 blur-3xl" />
      <div className="pointer-events-none absolute right-[8%] top-[18%] h-72 w-72 rounded-full glow-peach opacity-35" />
      <div className="pointer-events-none absolute right-[12%] bottom-[8%] h-80 w-80 rounded-full glow-green opacity-40" />

      <div className={`relative flex min-h-0 flex-1 flex-col ${HOME_PAGE_PADDING_CLASS}`}>
        {!controller.currentAgent ? (
          <Console
            agents={controller.agents}
            sessions={controller.sessions}
            currentAgentId={controller.currentAgentId}
            onSelectAgent={controller.handleAgentSelect}
            onOpenSession={controller.handleOpenSessionFromDirectory}
            onCreateAgent={controller.handleOpenCreateAgent}
            onEditAgent={controller.handleEditAgent}
            onDeleteAgent={controller.handleDeleteAgent}
          />
        ) : (
          <AgentWorkspace
            activeWorkspacePath={controller.activeWorkspacePath}
            agentCostSummary={controller.agentCostSummary}
            agents={controller.agents}
            currentAgent={controller.currentAgent}
            currentAgentId={controller.currentAgentId}
            currentAgentSessions={controller.currentAgentSessions}
            currentSession={controller.currentSession}
            currentSessionKey={controller.currentSessionKey}
            currentTodos={controller.currentTodos}
            editorWidthPercent={controller.editorWidthPercent}
            isEditorOpen={controller.isEditorOpen}
            isResizingEditor={controller.isResizingEditor}
            isSessionBusy={controller.isSessionBusy}
            onBackToDirectory={controller.handleBackToDirectory}
            onCloseWorkspacePane={controller.handleCloseWorkspacePane}
            onDeleteSession={controller.handleDeleteSession}
            onEditAgent={controller.handleEditAgent}
            onLoadingChange={controller.setIsSessionBusy}
            onNewSession={controller.handleNewSession}
            onOpenCreateAgent={controller.handleOpenCreateAgent}
            onOpenWorkspaceFile={controller.handleOpenWorkspaceFile}
            onSelectAgent={controller.handleAgentSelect}
            onSelectSession={controller.handleSessionSelect}
            onSessionSnapshotChange={controller.handleSessionSnapshotChange}
            onStartEditorResize={controller.handleStartEditorResize}
            onTodosChange={controller.setCurrentTodos}
            recentAgents={controller.recentAgents}
            sessionCostSummary={controller.sessionCostSummary}
            workspaceSplitRef={controller.workspaceSplitRef}
          />
        )}
      </div>

      <AgentOptions
        mode={controller.dialogMode}
        isOpen={controller.isDialogOpen}
        onClose={() => controller.setIsDialogOpen(false)}
        onSave={controller.handleSaveAgentOptions}
        onValidateName={controller.handleValidateAgentName}
        initialTitle={controller.dialogInitialTitle}
        initialOptions={controller.dialogInitialOptions}
      />
    </main>
  );
}
