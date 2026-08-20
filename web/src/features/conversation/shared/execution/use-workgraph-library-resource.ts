/**
 * INPUT: 当前 session_key、历史图 API 与 owner-scoped Workflow 目录 API。
 * OUTPUT: 可刷新/删除的历史 WorkGraph 与命名命令库资源。
 * POS: WorkGraph Surface 管理视图的唯一数据控制器；不创建 Workflow。
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  deleteWorkGraphWorkflowApi,
  getExecutionHistoryApi,
  getWorkGraphWorkflowsApi,
} from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import type { ExecutionView } from "@/types/conversation/execution";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

import { WORKGRAPH_WORKFLOWS_CHANGED_EVENT } from "./workgraph-distillation-intent";

export interface WorkGraphLibraryResource {
  deleteWorkflow: (workflowId: string) => Promise<void>;
  error: string | null;
  history: ExecutionView[];
  isLoading: boolean;
  refresh: () => void;
  workflows: WorkGraphWorkflow[];
}

export function useWorkGraphLibraryResource(
  sessionKey: string | null,
  enabled: boolean,
): WorkGraphLibraryResource {
  const [history, setHistory] = useState<ExecutionView[]>([]);
  const [workflows, setWorkflows] = useState<WorkGraphWorkflow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setLoading] = useState(false);
  const requestRef = useRef(0);

  const refresh = useCallback(async () => {
    if (!sessionKey || !enabled) {
      return;
    }
    const request = requestRef.current + 1;
    requestRef.current = request;
    setLoading(true);
    setError(null);
    try {
      const [nextHistory, nextWorkflows] = await Promise.all([
        getExecutionHistoryApi(sessionKey),
        getWorkGraphWorkflowsApi(),
      ]);
      if (requestRef.current !== request) {
        return;
      }
      setHistory(nextHistory);
      setWorkflows(nextWorkflows);
      window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
    } catch (reason) {
      if (requestRef.current === request) {
        setError(getErrorMessage(reason, "WorkGraph 资源读取失败"));
      }
    } finally {
      if (requestRef.current === request) {
        setLoading(false);
      }
    }
  }, [enabled, sessionKey]);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    void refresh();
  }, [enabled, refresh]);

  const deleteWorkflow = useCallback(async (workflowId: string) => {
    setError(null);
    try {
      await deleteWorkGraphWorkflowApi(workflowId);
      setWorkflows((current) => current.filter((item) => item.id !== workflowId));
      window.dispatchEvent(new CustomEvent(WORKGRAPH_WORKFLOWS_CHANGED_EVENT));
    } catch (reason) {
      setError(getErrorMessage(reason, "Workflow 删除失败"));
      throw reason;
    }
  }, []);

  return {
    deleteWorkflow,
    error,
    history,
    isLoading,
    refresh: () => void refresh(),
    workflows,
  };
}
