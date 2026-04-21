"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  clamp_home_editor_width_percent,
  HOME_EDITOR_DEFAULT_WIDTH_PERCENT,
} from "@/lib/layout/home-layout";
import { TodoItem } from "@/types/conversation/todo";
import { HomeWorkspaceControllerOptions } from "@/types/app/workspace";

export function useHomeWorkspaceController({
  current_agent_id,
}: HomeWorkspaceControllerOptions) {
  const [active_workspace_path, setActiveWorkspacePath] = useState<string | null>(null);
  const [is_editor_open, setIsEditorOpen] = useState(false);
  const [editor_width_percent, setEditorWidthPercent] = useState(HOME_EDITOR_DEFAULT_WIDTH_PERCENT);
  const [is_resizing_editor, setIsResizingEditor] = useState(false);
  const [current_todos, setCurrentTodos] = useState<TodoItem[]>([]);
  const [is_conversation_busy, setIsConversationBusy] = useState(false);
  const workspace_split_ref = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (current_agent_id) {
      return;
    }

    setActiveWorkspacePath(null);
    setIsEditorOpen(false);
    setCurrentTodos([]);
    setIsConversationBusy(false);
  }, [current_agent_id]);

  const handle_open_workspace_file = useCallback((path: string | null) => {
    // 对话区点击文件引用的语义应当始终是“打开这个文件”，
    // 不能因为重复点击同一路径就把编辑器反向关掉。
    setActiveWorkspacePath(path);
    setIsEditorOpen(Boolean(path));
  }, []);

  const handle_start_editor_resize = useCallback(() => {
    setIsResizingEditor(true);
  }, []);

  const handle_close_workspace_pane = useCallback(() => {
    setIsEditorOpen(false);
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
    handle_close_workspace_pane,
  };
}
