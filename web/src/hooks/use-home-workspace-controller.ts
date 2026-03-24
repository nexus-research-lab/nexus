"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { getConversationCostSummary } from "@/lib/agent-api";
import {
  clampHomeEditorWidthPercent,
  HOME_EDITOR_DEFAULT_WIDTH_PERCENT,
} from "@/lib/home-layout";
import { getAgentCostSummaryApi } from "@/lib/agent-manage-api";
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
  current_conversation,
}: HomeWorkspaceControllerOptions) {
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
    if (!current_agent_id || is_conversation_busy) {
      return;
    }

    let ignore = false;

    const loadAgentCostSummary = async () => {
      try {
        const nextSummary = await getAgentCostSummaryApi(current_agent_id);
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

    void loadAgentCostSummary();

    return () => {
      ignore = true;
    };
  }, [current_agent_id, is_conversation_busy]);

  useEffect(() => {
    if (!current_conversation?.session_key) {
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

    const loadConversationCostSummary = async () => {
      try {
        const nextSummary = await getConversationCostSummary(current_conversation.session_key);
        if (!ignore) {
          setConversationCostSummary(nextSummary);
        }
      } catch (error) {
        console.error("Failed to load conversation cost summary:", error);
        if (!ignore) {
          setConversationCostSummary({
            ...EMPTY_CONVERSATION_COST_SUMMARY,
            agent_id: current_agent_id ?? "",
            session_key: current_conversation.session_key,
            session_id: current_conversation.session_id ?? "",
          });
        }
      }
    };

    void loadConversationCostSummary();

    return () => {
      ignore = true;
    };
  }, [current_conversation?.session_id, current_conversation?.session_key, current_agent_id, is_conversation_busy]);

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

    const handleMouseMove = (event: MouseEvent) => {
      const container = workspace_split_ref.current;
      if (!container) {
        return;
      }

      const bounds = container.getBoundingClientRect();
      const nextPercent = ((event.clientX - bounds.left) / bounds.width) * 100;
      setEditorWidthPercent(clampHomeEditorWidthPercent(nextPercent));
    };

    const handleMouseUp = () => {
      setIsResizingEditor(false);
    };

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
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
