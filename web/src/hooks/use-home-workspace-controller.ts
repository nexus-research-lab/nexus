"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { get_conversation_cost_summary } from "@/lib/agent-api";
import {
  clamp_home_editor_width_percent,
  HOME_EDITOR_DEFAULT_WIDTH_PERCENT,
} from "@/lib/home-layout";
import { get_agent_cost_summary_api } from "@/lib/agent-manage-api";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { AgentCostSummary, ConversationCostSummary } from "@/types/cost";
import { TodoItem } from "@/types/todo";
import { HomeWorkspaceControllerOptions } from "@/types/workspace";

const EMPTY_CONVERSATION_COST_SUMMARY: ConversationCostSummary = {
  agent_id: "",
  session_key: "",
  session_id: "",
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_tokens: 0,
  total_cache_creation_input_tokens: 0,
  total_cache_read_input_tokens: 0,
  total_cost_usd: 0,
  completed_rounds: 0,
  error_rounds: 0,
  last_round_id: null,
  last_run_duration_ms: null,
  last_run_cost_usd: null,
};

const EMPTY_AGENT_COST_SUMMARY: AgentCostSummary = {
  agent_id: "",
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_tokens: 0,
  total_cache_creation_input_tokens: 0,
  total_cache_read_input_tokens: 0,
  total_cost_usd: 0,
  completed_rounds: 0,
  error_rounds: 0,
  cost_sessions: 0,
};

export function useHomeWorkspaceController({
  current_agent_id,
  current_agent_conversation,
  current_agent_session_identity,
}: HomeWorkspaceControllerOptions) {
  const clear_workspace_agent = useWorkspaceFilesStore((state) => state.clear_agent);
  const refresh_workspace_files = useWorkspaceFilesStore((state) => state.refresh_files);
  const [active_workspace_path, setActiveWorkspacePath] = useState<string | null>(null);
  const [is_editor_open, setIsEditorOpen] = useState(false);
  const [editor_width_percent, setEditorWidthPercent] = useState(HOME_EDITOR_DEFAULT_WIDTH_PERCENT);
  const [is_resizing_editor, setIsResizingEditor] = useState(false);
  const [current_todos, setCurrentTodos] = useState<TodoItem[]>([]);
  const [is_conversation_busy, setIsConversationBusy] = useState(false);
  const [conversation_cost_summary, setConversationCostSummary] = useState<ConversationCostSummary>(
    EMPTY_CONVERSATION_COST_SUMMARY,
  );
  const [agent_cost_summary, setAgentCostSummary] = useState<AgentCostSummary>(
    EMPTY_AGENT_COST_SUMMARY,
  );
  const workspace_split_ref = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (current_agent_id) {
      return;
    }

    setActiveWorkspacePath(null);
    setIsEditorOpen(false);
    setCurrentTodos([]);
    setIsConversationBusy(false);
    setConversationCostSummary(EMPTY_CONVERSATION_COST_SUMMARY);
    setAgentCostSummary(EMPTY_AGENT_COST_SUMMARY);
  }, [current_agent_id]);

  useEffect(() => {
    if (!current_agent_id) {
      return;
    }
    if (is_conversation_busy) {
      return;
    }

    let ignore = false;

    const load_workspace_files = async () => {
      try {
        await refresh_workspace_files(current_agent_id);
      } catch (error) {
        console.error("Failed to load workspace files:", error);
        if (!ignore) {
          clear_workspace_agent(current_agent_id);
        }
      }
    };

    void load_workspace_files();

    return () => {
      ignore = true;
    };
  }, [
    clear_workspace_agent,
    current_agent_id,
    current_agent_session_identity?.session_key,
    is_conversation_busy,
    refresh_workspace_files,
  ]);

  useEffect(() => {
    if (!current_agent_id || is_conversation_busy) {
      return;
    }

    let ignore = false;

    const load_agent_cost_summary = async () => {
      try {
        const nextSummary = await get_agent_cost_summary_api(current_agent_id);
        if (!ignore) {
          setAgentCostSummary(nextSummary);
        }
      } catch (error) {
        console.error("Failed to load agent cost summary:", error);
        if (!ignore) {
          setAgentCostSummary({
            ...EMPTY_AGENT_COST_SUMMARY,
            agent_id: current_agent_id,
          });
        }
      }
    };

    void load_agent_cost_summary();

    return () => {
      ignore = true;
    };
  }, [current_agent_id, is_conversation_busy]);

  useEffect(() => {
    const current_session_key = current_agent_session_identity?.session_key ?? null;
    if (!current_session_key) {
      setConversationCostSummary({
        ...EMPTY_CONVERSATION_COST_SUMMARY,
        agent_id: current_agent_id ?? "",
      });
      return;
    }
    if (is_conversation_busy) {
      return;
    }

    let ignore = false;

    const load_conversation_cost_summary = async () => {
      try {
        const nextSummary = await get_conversation_cost_summary(current_session_key);
        if (!ignore) {
          setConversationCostSummary(nextSummary);
        }
      } catch (error) {
        console.error("Failed to load conversation cost summary:", error);
        if (!ignore) {
          setConversationCostSummary({
            ...EMPTY_CONVERSATION_COST_SUMMARY,
            agent_id: current_agent_id ?? "",
            session_key: current_session_key,
            session_id: current_agent_conversation?.session_id ?? "",
          });
        }
      }
    };

    void load_conversation_cost_summary();

    return () => {
      ignore = true;
    };
  }, [
    current_agent_conversation?.session_id,
    current_agent_id,
    current_agent_session_identity?.session_key,
    is_conversation_busy,
  ]);

  const handle_open_workspace_file = useCallback((path: string | null) => {
    setActiveWorkspacePath((currentPath) => {
      if (path && currentPath === path && is_editor_open) {
        setIsEditorOpen(false);
        return null;
      }

      setIsEditorOpen(Boolean(path));
      return path;
    });
  }, [is_editor_open]);

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
    conversation_cost_summary,
    agent_cost_summary,
    workspace_split_ref,
    set_current_todos: setCurrentTodos,
    set_is_conversation_busy: setIsConversationBusy,
    handle_open_workspace_file,
    handle_start_editor_resize,
    handle_close_workspace_pane,
  };
}
