"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  clampHomeSidePanelWidthPercent,
  HOME_SIDE_PANEL_DEFAULT_WIDTH_PERCENT,
} from "@/lib/layout/home-layout";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { TodoItem } from "@/types/conversation/todo";
import { HomeWorkspaceControllerOptions } from "@/types/app/workspace";

export function useHomeWorkspaceController({
  currentAgentId,
  workspaceAgentIds,
}: HomeWorkspaceControllerOptions) {
  const agentResetKey = currentAgentId ? "has-agent" : "no-agent";
  const [activeWorkspacePath, setActiveWorkspacePath] = useResettableState<string | null>(null, agentResetKey);
  const [sidePanelWidthPercent, setSidePanelWidthPercent] = useState(HOME_SIDE_PANEL_DEFAULT_WIDTH_PERCENT);
  const [isResizingSidePanel, setIsResizingSidePanel] = useState(false);
  const [currentTodos, setCurrentTodos] = useResettableState<TodoItem[]>([], agentResetKey);
  const surfaceSplitRef = useRef<HTMLElement | null>(null);
  const filesByAgent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const refreshFiles = useWorkspaceFilesStore((state) => state.refresh_files);
  const requestOpenAgent = useWorkspaceFilesStore((state) => state.request_open_agent);

  const preloadWorkspaceAgentIds = useMemo(() => {
    const agentIds = new Set<string>();
    if (currentAgentId) {
      agentIds.add(currentAgentId);
    }
    for (const agentId of workspaceAgentIds ?? []) {
      const normalizedAgentId = agentId.trim();
      if (normalizedAgentId) {
        agentIds.add(normalizedAgentId);
      }
    }
    return Array.from(agentIds);
  }, [currentAgentId, workspaceAgentIds]);

  useEffect(() => {
    if (preloadWorkspaceAgentIds.length === 0) {
      return;
    }

    const loadWorkspaceFiles = async () => {
      const missingAgentIds = preloadWorkspaceAgentIds.filter(
        (agentId) => !filesByAgent[agentId],
      );
      await Promise.all(
        missingAgentIds.map(async (agentId) => {
          // 消息区文件按钮依赖这份缓存解析路径；失败由工作区展示，不阻断聊天。
          await refreshFiles(agentId).catch(() => undefined);
        }),
      );
    };

    void loadWorkspaceFiles();
  }, [filesByAgent, preloadWorkspaceAgentIds, refreshFiles]);

  const handleOpenWorkspaceFile = useCallback((path: string | null, workspaceAgentId?: string | null) => {
    // 对话区点击文件引用的语义应当始终是“打开这个文件”，
    // 不能因为重复点击同一路径就把编辑器反向关掉。
    setActiveWorkspacePath(path);
    // 资产带上归属 Agent 时（群聊里文件可能属于别的 Agent），请求 workspace 面板切到它，
    // 否则会在当前选中 Agent 名下取不到文件 → “资源不存在”。
    if (path && workspaceAgentId?.trim()) {
      requestOpenAgent(workspaceAgentId);
    }
  }, [requestOpenAgent, setActiveWorkspacePath]);

  const handleStartSidePanelResize = useCallback(() => {
    setIsResizingSidePanel(true);
  }, []);

  useEffect(() => {
    if (!isResizingSidePanel) {
      return;
    }

    const handleMouseMove = (event: MouseEvent) => {
      const container = surfaceSplitRef.current;
      if (!container) {
        return;
      }

      const bounds = container.getBoundingClientRect();
      const nextPercent = ((bounds.right - event.clientX) / bounds.width) * 100;
      setSidePanelWidthPercent(clampHomeSidePanelWidthPercent(nextPercent));
    };

    const handleMouseUp = () => {
      setIsResizingSidePanel(false);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizingSidePanel]);

  return {
    activeWorkspacePath,
    sidePanelWidthPercent,
    isResizingSidePanel,
    currentTodos,
    surfaceSplitRef,
    setCurrentTodos,
    handleOpenWorkspaceFile,
    handleStartSidePanelResize,
  };
}
