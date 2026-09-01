/**
 * INPUT: 当前 owner scope、任务主列表和权限辅助读取。
 * OUTPUT: 代次隔离的主快照、访问失效封闭状态与显式同 scope 重新验证入口。
 * POS: Scheduled 资源真相边界；403 重试期间保持敏感快照为空，成功后才解除访问 fence。
 */
"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import { resolveAgentId } from "@/config/runtime-options";
import {
  listAutomationPermissionRequestsApi,
  listScheduledTasksApi,
} from "@/lib/api/capability/scheduled-task-api";
import { getResourceFailure, type ResourceFailure } from "@/lib/error-message";
import type { AutomationPermissionRequest } from "@/types/capability/scheduled-task/permission";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

interface RefreshScheduledTasksOptions {
  includePermissions?: boolean;
  silent?: boolean;
}

export interface ScheduledTasksPrimarySnapshot {
  items: ScheduledTaskItem[];
}

interface ScheduledTasksResourceState {
  failure: ResourceFailure | null;
  hasSnapshot: boolean;
  isLoading: boolean;
  isPermissionLoading: boolean;
  items: ScheduledTaskItem[];
  permissionFailure: ResourceFailure | null;
}

class ScheduledTasksRefreshSupersededError extends Error {
  constructor() {
    super("任务列表已由更新的请求接管");
    this.name = "ScheduledTasksRefreshSupersededError";
  }
}

function upsertScheduledTask(
  items: ScheduledTaskItem[],
  nextTask: ScheduledTaskItem,
): ScheduledTaskItem[] {
  const taskIndex = items.findIndex((item) => item.job_id === nextTask.job_id);
  if (taskIndex < 0) {
    return [nextTask, ...items];
  }
  return items.map((item, index) => (index === taskIndex ? nextTask : item));
}

function mergePermissionRequests(
  items: ScheduledTaskItem[],
  permissionRequests: AutomationPermissionRequest[],
): ScheduledTaskItem[] {
  const requestsById = new Map(
    permissionRequests.map((item) => [item.request_id, item]),
  );
  return items.map((task) => ({
    ...task,
    pending_permission_request: task.pending_permission_request_id
      ? requestsById.get(task.pending_permission_request_id) ?? null
      : null,
  }));
}

export function useScheduledTasksResource(scopeKey: string | null) {
  const [state, setState] = useState<ScheduledTasksResourceState>({
    failure: null,
    hasSnapshot: false,
    isLoading: scopeKey !== null,
    isPermissionLoading: scopeKey !== null,
    items: [],
    permissionFailure: null,
  });
  const foregroundRequestsRef = useRef(new Set<symbol>());
  const accessFailureRef = useRef<ResourceFailure | null>(null);
  const accessInvalidatedRef = useRef(false);
  const activeScopeKeyRef = useRef(scopeKey);
  const isMountedRef = useRef(false);
  const lastSuccessfulPermissionReadVersionRef = useRef(0);
  const lastSuccessfulReadVersionRef = useRef(0);
  const permissionRequestVersionRef = useRef(0);
  const permissionSnapshotRef = useRef<{
    hasSnapshot: boolean;
    requests: AutomationPermissionRequest[];
  }>({ hasSnapshot: false, requests: [] });
  const requestVersionRef = useRef(0);

  const isCurrentRequest = useCallback((version: number): boolean => (
    isMountedRef.current && requestVersionRef.current === version
  ), []);

  const isCurrentPermissionRequest = useCallback((version: number): boolean => (
    isMountedRef.current && permissionRequestVersionRef.current === version
  ), []);
  const isAccessInvalidated = useCallback((): boolean => (
    accessInvalidatedRef.current
  ), []);

  const invalidateAccess = useCallback((failure: ResourceFailure): void => {
    if (!failure.access) {
      return;
    }
    accessFailureRef.current = failure;
    accessInvalidatedRef.current = true;
    requestVersionRef.current += 1;
    permissionRequestVersionRef.current += 1;
    permissionSnapshotRef.current = { hasSnapshot: false, requests: [] };
    foregroundRequestsRef.current.clear();
    setState({
      failure,
      hasSnapshot: false,
      isLoading: false,
      isPermissionLoading: false,
      items: [],
      permissionFailure: null,
    });
  }, []);

  const refreshPermissionRequests = useCallback(async (): Promise<
    AutomationPermissionRequest[]
  > => {
    if (accessInvalidatedRef.current) {
      throw new ScheduledTasksRefreshSupersededError();
    }
    const requestScopeKey = activeScopeKeyRef.current;
    const version = permissionRequestVersionRef.current + 1;
    permissionRequestVersionRef.current = version;
    setState((current) => ({
      ...current,
      isPermissionLoading: true,
    }));
    try {
      const permissionRequests = await listAutomationPermissionRequestsApi({
        status: "actionable",
      });
      if (!isCurrentPermissionRequest(version)) {
        throw new ScheduledTasksRefreshSupersededError();
      }
      lastSuccessfulPermissionReadVersionRef.current = version;
      permissionSnapshotRef.current = {
        hasSnapshot: true,
        requests: permissionRequests,
      };
      setState((current) => ({
        ...current,
        isPermissionLoading: false,
        items: mergePermissionRequests(current.items, permissionRequests),
        permissionFailure: null,
      }));
      return permissionRequests;
    } catch (error) {
      if (error instanceof ScheduledTasksRefreshSupersededError) {
        throw error;
      }
      const failure = getResourceFailure(error, "加载任务权限状态失败");
      const sameScope = activeScopeKeyRef.current === requestScopeKey;
      const current = sameScope && isCurrentPermissionRequest(version);
      const mustHideStaleAccess = sameScope && Boolean(failure.access)
        && lastSuccessfulPermissionReadVersionRef.current < version;
      if (current || mustHideStaleAccess) {
        if (failure.access) {
          // 权限辅助读取失效只隐藏审批详情，不能拖垮任务主快照。
          permissionSnapshotRef.current = { hasSnapshot: true, requests: [] };
        }
        setState((previous) => ({
          ...previous,
          isPermissionLoading: current ? false : previous.isPermissionLoading,
          items: failure.access
            ? mergePermissionRequests(previous.items, [])
            : previous.items,
          permissionFailure: failure,
        }));
      }
      throw error;
    }
  }, [isCurrentPermissionRequest]);

  const refresh = useCallback(async (
    options: RefreshScheduledTasksOptions = {},
  ): Promise<ScheduledTasksPrimarySnapshot> => {
    if (accessInvalidatedRef.current) {
      throw new ScheduledTasksRefreshSupersededError();
    }
    const requestScopeKey = activeScopeKeyRef.current;
    if (options.includePermissions !== false) {
      // 辅助权限请求独立落状态；它再慢或失败，也不延迟主列表 Promise。
      void refreshPermissionRequests().catch(() => undefined);
    }
    const version = requestVersionRef.current + 1;
    requestVersionRef.current = version;
    const foregroundRequest = options.silent ? null : Symbol("scheduled-tasks-refresh");
    if (foregroundRequest) {
      foregroundRequestsRef.current.add(foregroundRequest);
    }
    if (!options.silent) {
      setState((current) => ({
        ...current,
        failure: current.failure?.access ? current.failure : null,
        isLoading: true,
      }));
    }
    const finishForegroundRequest = (): void => {
      if (foregroundRequest) {
        foregroundRequestsRef.current.delete(foregroundRequest);
        if (
          isMountedRef.current
          && activeScopeKeyRef.current === requestScopeKey
        ) {
          setState((current) => ({
            ...current,
            isLoading: foregroundRequestsRef.current.size > 0,
          }));
        }
      }
    };
    let tasks: ScheduledTaskItem[];
    try {
      tasks = await listScheduledTasksApi();
    } catch (error) {
      const failure = getResourceFailure(error, "加载定时任务失败");
      const sameScope = activeScopeKeyRef.current === requestScopeKey;
      const current = sameScope && isCurrentRequest(version);
      const mustHideStaleAccess = sameScope && Boolean(failure.access)
        && lastSuccessfulReadVersionRef.current < version;
      if (failure.access && (current || mustHideStaleAccess)) {
        // 主资源访问身份已失效；旧 owner 的任何快照都不能跨登录代次复用。
        invalidateAccess(failure);
      } else if (current) {
        setState((previous) => ({
          ...previous,
          failure,
          hasSnapshot: previous.hasSnapshot,
          items: previous.items,
        }));
      }
      finishForegroundRequest();
      throw error;
    }

    finishForegroundRequest();
    if (!isCurrentRequest(version) || accessInvalidatedRef.current) {
      throw new ScheduledTasksRefreshSupersededError();
    }
    accessFailureRef.current = null;
    lastSuccessfulReadVersionRef.current = version;
    setState((current) => {
      const permissionSnapshot = permissionSnapshotRef.current;
      const retainedPermissionRequests = current.items.flatMap((item) => (
        item.pending_permission_request ? [item.pending_permission_request] : []
      ));
      return {
        ...current,
        failure: null,
        hasSnapshot: true,
        items: mergePermissionRequests(
          tasks,
          permissionSnapshot.hasSnapshot
            ? permissionSnapshot.requests
            : retainedPermissionRequests,
        ),
      };
    });
    return { items: tasks };
  }, [invalidateAccess, isCurrentRequest, refreshPermissionRequests]);

  const revalidateAccess = useCallback(async (): Promise<
    ScheduledTasksPrimarySnapshot
  > => {
    if (!accessInvalidatedRef.current) {
      return refresh();
    }
    const requestScopeKey = activeScopeKeyRef.current;
    if (!requestScopeKey) {
      throw new ScheduledTasksRefreshSupersededError();
    }

    // 这是访问失效后的唯一显式读取入口。先推进主/辅助请求代次并继续
    // 保持敏感快照为空；不能为了“试一下”临时恢复旧 owner 数据。
    const version = requestVersionRef.current + 1;
    requestVersionRef.current = version;
    permissionRequestVersionRef.current += 1;
    permissionSnapshotRef.current = { hasSnapshot: false, requests: [] };
    foregroundRequestsRef.current.clear();
    setState((current) => ({
      ...current,
      failure: accessFailureRef.current ?? current.failure,
      hasSnapshot: false,
      isLoading: true,
      isPermissionLoading: false,
      items: [],
      permissionFailure: null,
    }));

    let tasks: ScheduledTaskItem[];
    try {
      tasks = await listScheduledTasksApi();
    } catch (error) {
      const sameRequest = activeScopeKeyRef.current === requestScopeKey
        && isCurrentRequest(version);
      if (sameRequest) {
        const failure = getResourceFailure(error, "重新验证任务访问权限失败");
        const retainedAccess = failure.access
          ?? accessFailureRef.current?.access
          ?? "forbidden";
        const retainedFailure: ResourceFailure = {
          access: retainedAccess,
          message: failure.message,
        };
        accessFailureRef.current = retainedFailure;
        setState({
          failure: retainedFailure,
          hasSnapshot: false,
          isLoading: false,
          isPermissionLoading: false,
          items: [],
          permissionFailure: null,
        });
      }
      throw error;
    }

    if (
      activeScopeKeyRef.current !== requestScopeKey
      || !isCurrentRequest(version)
    ) {
      throw new ScheduledTasksRefreshSupersededError();
    }
    // 只有同 owner 的权威主列表完整返回，才允许恢复命令控制器和页面。
    accessInvalidatedRef.current = false;
    accessFailureRef.current = null;
    lastSuccessfulReadVersionRef.current = version;
    setState({
      failure: null,
      hasSnapshot: true,
      isLoading: false,
      isPermissionLoading: false,
      items: tasks,
      permissionFailure: null,
    });
    void refreshPermissionRequests().catch(() => undefined);
    return { items: tasks };
  }, [isCurrentRequest, refresh, refreshPermissionRequests]);

  const commitItems = useCallback((
    update: (currentItems: ScheduledTaskItem[]) => ScheduledTaskItem[],
    expectedScopeKey: string | null,
  ): void => {
    if (
      !expectedScopeKey
      || activeScopeKeyRef.current !== expectedScopeKey
      || accessInvalidatedRef.current
    ) {
      return;
    }
    // 本地命令结果推进请求代次，避免更早的列表响应回滚已确认状态。
    requestVersionRef.current += 1;
    foregroundRequestsRef.current.clear();
    setState((current) => {
      // 访问已失效时，迟到的旧命令响应不得重新放回已隐藏快照。
      if (current.failure?.access) {
        return current;
      }
      const updatedItems = update(current.items);
      return {
        ...current,
        failure: null,
        hasSnapshot: true,
        isLoading: false,
        items: permissionSnapshotRef.current.hasSnapshot
          ? mergePermissionRequests(
              updatedItems,
              permissionSnapshotRef.current.requests,
            )
          : updatedItems,
      };
    });
  }, []);

  const upsertTask = useCallback((
    task: ScheduledTaskItem,
    expectedScopeKey: string | null,
  ): void => {
    commitItems(
      (currentItems) => upsertScheduledTask(currentItems, task),
      expectedScopeKey,
    );
  }, [commitItems]);

  const removeTask = useCallback((
    jobId: string,
    expectedScopeKey: string | null,
  ): void => {
    commitItems((currentItems) => (
      currentItems.filter((item) => item.job_id !== jobId)
    ), expectedScopeKey);
  }, [commitItems]);

  useLayoutEffect(() => {
    if (activeScopeKeyRef.current === scopeKey) {
      return;
    }
    activeScopeKeyRef.current = scopeKey;
    accessFailureRef.current = null;
    accessInvalidatedRef.current = false;
    requestVersionRef.current += 1;
    permissionRequestVersionRef.current += 1;
    permissionSnapshotRef.current = { hasSnapshot: false, requests: [] };
    foregroundRequestsRef.current.clear();
    setState({
      failure: null,
      hasSnapshot: false,
      isLoading: scopeKey !== null,
      isPermissionLoading: scopeKey !== null,
      items: [],
      permissionFailure: null,
    });
  }, [scopeKey]);

  useEffect(() => {
    const foregroundRequests = foregroundRequestsRef.current;
    isMountedRef.current = true;
    if (!scopeKey) {
      return () => {
        isMountedRef.current = false;
        requestVersionRef.current += 1;
        permissionRequestVersionRef.current += 1;
        foregroundRequests.clear();
      };
    }
    void refresh().catch((error: unknown) => {
      console.debug("[scheduled-tasks] Initial load failed:", error);
    });
    return () => {
      isMountedRef.current = false;
      requestVersionRef.current += 1;
      permissionRequestVersionRef.current += 1;
      foregroundRequests.clear();
    };
  }, [refresh, scopeKey]);

  return {
    ...state,
    agentId: resolveAgentId(),
    refresh,
    refreshPermissionRequests,
    revalidateAccess,
    invalidateAccess,
    isAccessInvalidated,
    removeTask,
    upsertTask,
  };
}
