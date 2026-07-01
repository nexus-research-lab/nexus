"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  clamp_home_editor_width_percent,
  HOME_EDITOR_DEFAULT_WIDTH_PERCENT,
} from "@/lib/layout/home-layout";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { TodoItem } from "@/types/conversation/todo";
import { HomeWorkspaceControllerOptions } from "@/types/app/workspace";

export function useHomeWorkspaceController({
  current_agent_id,
  workspace_agent_ids,
}: HomeWorkspaceControllerOptions) {
  const agent_reset_key = current_agent_id ? "has-agent" : "no-agent";
  const [active_workspace_path, setActiveWorkspacePath] = useResettableState<string | null>(null, agent_reset_key);
  const [is_editor_open, setIsEditorOpen] = useResettableState(false, agent_reset_key);
  const [editor_width_percent, setEditorWidthPercent] = useState(HOME_EDITOR_DEFAULT_WIDTH_PERCENT);
  const [is_resizing_editor, setIsResizingEditor] = useState(false);
  const [current_todos, setCurrentTodos] = useResettableState<TodoItem[]>([], agent_reset_key);
  const [is_conversation_busy, setIsConversationBusy] = useResettableState(false, agent_reset_key);
  const workspace_split_ref = useRef<HTMLElement | null>(null);
  const files_by_agent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const refresh_files = useWorkspaceFilesStore((state) => state.refresh_files);

  const preload_workspace_agent_ids = useMemo(() => {
    const agent_ids = new Set<string>();
    if (current_agent_id) {
      agent_ids.add(current_agent_id);
    }
    for (const agent_id of workspace_agent_ids ?? []) {
      const normalized_agent_id = agent_id.trim();
      if (normalized_agent_id) {
        agent_ids.add(normalized_agent_id);
      }
    }
    return Array.from(agent_ids);
  }, [current_agent_id, workspace_agent_ids]);

  useEffect(() => {
    if (preload_workspace_agent_ids.length === 0) {
      return;
    }

    const load_workspace_files = async () => {
      const missing_agent_ids = preload_workspace_agent_ids.filter(
        (agent_id) => !files_by_agent[agent_id],
      );
      await Promise.all(
        missing_agent_ids.map(async (agent_id) => {
          // 中文注释：消息区的文件按钮依赖这份缓存做路径解析；
          // 预加载失败时保留 workspace 面板自身的错误展示，不阻断聊天。
          await refresh_files(agent_id).catch(() => undefined);
        }),
      );
    };

    void load_workspace_files();
  }, [files_by_agent, preload_workspace_agent_ids, refresh_files]);

  const handle_open_workspace_file = useCallback((path: string | null) => {
    // 对话区点击文件引用的语义应当始终是“打开这个文件”，
    // 不能因为重复点击同一路径就把编辑器反向关掉。
    setActiveWorkspacePath(path);
    setIsEditorOpen(Boolean(path));
  }, []);

  const handle_start_editor_resize = useCallback(() => {
    setIsResizingEditor(true);
  }, []);

  useEffect(() => {
    if (!is_resizing_editor) {
      return;
    }

    const handle_mouse_move = (event: MouseEvent) => {
      const container = workspace_split_ref.current;
      if (!container) {
        return;
      }

      const bounds = container.getBoundingClientRect();
      const nextPercent = ((bounds.right - event.clientX) / bounds.width) * 100;
      setEditorWidthPercent(clamp_home_editor_width_percent(nextPercent));
    };

    const handle_mouse_up = () => {
      setIsResizingEditor(false);
    };

    window.addEventListener("mousemove", handle_mouse_move);
    window.addEventListener("mouseup", handle_mouse_up);

    return () => {
      window.removeEventListener("mousemove", handle_mouse_move);
      window.removeEventListener("mouseup", handle_mouse_up);
    };
  }, [is_resizing_editor]);

  return {
    active_workspace_path,
    is_editor_open,
    editor_width_percent,
    is_resizing_editor,
    current_todos,
    is_conversation_busy,
    workspace_split_ref,
    set_current_todos: setCurrentTodos,
    set_is_conversation_busy: setIsConversationBusy,
    handle_open_workspace_file,
    handle_start_editor_resize,
  };
}
