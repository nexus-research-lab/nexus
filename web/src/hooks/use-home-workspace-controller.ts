"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { getSessionCostSummary } from "@/lib/agent-api";
import {
  clampHomeEditorWidthPercent,
  HOME_EDITOR_DEFAULT_WIDTH_PERCENT,
} from "@/lib/home-layout";
import { getAgentCostSummaryApi } from "@/lib/agent-manage-api";
import { TodoItem } from "@/components/workspace/agent-task-widget";
import { AgentCostSummary, SessionCostSummary } from "@/types/cost";
import { Session } from "@/types/session";

const EMPTY_SESSION_COST_SUMMARY: SessionCostSummary = {
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

interface UseHomeWorkspaceControllerOptions {
  currentAgentId: string | null;
  currentSession: Session | null;
}

export function useHomeWorkspaceController({
  currentAgentId,
  currentSession,
}: UseHomeWorkspaceControllerOptions) {
  const [activeWorkspacePath, setActiveWorkspacePath] = useState<string | null>(null);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [editorWidthPercent, setEditorWidthPercent] = useState(HOME_EDITOR_DEFAULT_WIDTH_PERCENT);
  const [isResizingEditor, setIsResizingEditor] = useState(false);
  const [currentTodos, setCurrentTodos] = useState<TodoItem[]>([]);
  const [isSessionBusy, setIsSessionBusy] = useState(false);
  const [sessionCostSummary, setSessionCostSummary] = useState<SessionCostSummary>(
    EMPTY_SESSION_COST_SUMMARY,
  );
  const [agentCostSummary, setAgentCostSummary] = useState<AgentCostSummary>(
    EMPTY_AGENT_COST_SUMMARY,
  );
  const workspaceSplitRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (currentAgentId) {
      return;
    }

    setActiveWorkspacePath(null);
    setIsEditorOpen(false);
    setCurrentTodos([]);
    setIsSessionBusy(false);
    setSessionCostSummary(EMPTY_SESSION_COST_SUMMARY);
    setAgentCostSummary(EMPTY_AGENT_COST_SUMMARY);
  }, [currentAgentId]);

  useEffect(() => {
    if (!currentAgentId || isSessionBusy) {
      return;
    }

    let ignore = false;

    const loadAgentCostSummary = async () => {
      try {
        const nextSummary = await getAgentCostSummaryApi(currentAgentId);
        if (!ignore) {
          setAgentCostSummary(nextSummary);
        }
      } catch (error) {
        console.error("Failed to load agent cost summary:", error);
        if (!ignore) {
          setAgentCostSummary({
            ...EMPTY_AGENT_COST_SUMMARY,
            agent_id: currentAgentId,
          });
        }
      }
    };

    void loadAgentCostSummary();

    return () => {
      ignore = true;
    };
  }, [currentAgentId, isSessionBusy]);

  useEffect(() => {
    if (!currentSession?.session_key) {
      setSessionCostSummary({
        ...EMPTY_SESSION_COST_SUMMARY,
        agent_id: currentAgentId ?? "",
      });
      return;
    }
    if (isSessionBusy) {
      return;
    }

    let ignore = false;

    const loadSessionCostSummary = async () => {
      try {
        const nextSummary = await getSessionCostSummary(currentSession.session_key);
        if (!ignore) {
          setSessionCostSummary(nextSummary);
        }
      } catch (error) {
        console.error("Failed to load session cost summary:", error);
        if (!ignore) {
          setSessionCostSummary({
            ...EMPTY_SESSION_COST_SUMMARY,
            agent_id: currentAgentId ?? "",
            session_key: currentSession.session_key,
            session_id: currentSession.session_id ?? "",
          });
        }
      }
    };

    void loadSessionCostSummary();

    return () => {
      ignore = true;
    };
  }, [currentSession?.session_id, currentSession?.session_key, currentAgentId, isSessionBusy]);

  const handleOpenWorkspaceFile = useCallback((path: string | null) => {
    setActiveWorkspacePath((currentPath) => {
      if (path && currentPath === path && isEditorOpen) {
        setIsEditorOpen(false);
        return null;
      }

      setIsEditorOpen(Boolean(path));
      return path;
    });
  }, [isEditorOpen]);

  const handleStartEditorResize = useCallback(() => {
    setIsResizingEditor(true);
  }, []);

  const handleCloseWorkspacePane = useCallback(() => {
    setIsEditorOpen(false);
  }, []);

  useEffect(() => {
    if (!isResizingEditor) {
      return;
    }

    const handleMouseMove = (event: MouseEvent) => {
      const container = workspaceSplitRef.current;
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
  }, [isResizingEditor]);

  return {
    activeWorkspacePath,
    isEditorOpen,
    editorWidthPercent,
    isResizingEditor,
    currentTodos,
    isSessionBusy,
    sessionCostSummary,
    agentCostSummary,
    workspaceSplitRef,
    setCurrentTodos,
    setIsSessionBusy,
    handleOpenWorkspaceFile,
    handleStartEditorResize,
    handleCloseWorkspacePane,
  };
}
