/**
 * INPUT: 公共骨架图的 exact session/Execution、当前打开的节点 ID 与骨架刷新代次。
 * OUTPUT: 该节点的完整运行事实（摘要、NodeRun 历史、Artifact 与直接子节点）读取状态。
 * POS: 节点/边检查器的按需详情资源；骨架只承载结构，随运行时长增长的内容只在这里读取。
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { getExecutionGraphNodeDetailApi } from "@/lib/api/conversation/execution-api";
import type { ExecutionGraphNodeDetailView } from "@/types/conversation/execution";

/** 读取单个节点详情；缺省表示图数据已内联（草图预览），检查器直接使用图内字段。 */
export type ExecutionNodeDetailLoader = (nodeId: string) => Promise<ExecutionGraphNodeDetailView>;

export interface ExecutionNodeDetailState {
  detail: ExecutionGraphNodeDetailView | null;
  failed: boolean;
  loading: boolean;
  retry: () => void;
}

interface ExecutionNodeDetailSnapshot {
  detail: ExecutionGraphNodeDetailView | null;
  failed: boolean;
  key: string | null;
  loading: boolean;
}

const IDLE_SNAPSHOT: ExecutionNodeDetailSnapshot = { detail: null, failed: false, key: null, loading: false };

/** 为 exact session/Execution 构造节点详情读取函数。 */
export function useExecutionNodeDetailLoader(
  sessionKey: string | null | undefined,
  executionId: string | null | undefined,
): ExecutionNodeDetailLoader | undefined {
  const load = useCallback(
    (nodeId: string) => getExecutionGraphNodeDetailApi(sessionKey ?? "", executionId ?? "", nodeId),
    [executionId, sessionKey],
  );
  return sessionKey && executionId ? load : undefined;
}

/**
 * 打开节点时读取详情；骨架每次刷新（revision 变化）都重读当前节点，使检查器与画布
 * 状态一致。刷新期间保留同一节点上一次成功的详情，避免内容闪烁；切换节点立即丢弃。
 */
export function useExecutionNodeDetail(
  loader: ExecutionNodeDetailLoader | undefined,
  nodeId: string | null,
  revision: unknown,
): ExecutionNodeDetailState {
  const [snapshot, setSnapshot] = useState<ExecutionNodeDetailSnapshot>(IDLE_SNAPSHOT);
  const [attempt, setAttempt] = useState(0);
  const loaderRef = useRef(loader);
  loaderRef.current = loader;
  const enabled = Boolean(loader && nodeId);

  useEffect(() => {
    const load = loaderRef.current;
    if (!load || !nodeId) return;
    let current = true;
    setSnapshot((previous) => ({
      detail: previous.key === nodeId ? previous.detail : null,
      failed: false,
      key: nodeId,
      loading: true,
    }));
    load(nodeId).then(
      (detail) => {
        if (current) setSnapshot({ detail, failed: false, key: nodeId, loading: false });
      },
      () => {
        if (current) {
          setSnapshot((previous) => ({
            detail: previous.key === nodeId ? previous.detail : null,
            failed: true,
            key: nodeId,
            loading: false,
          }));
        }
      },
    );
    return () => { current = false; };
  }, [attempt, enabled, nodeId, revision]);

  const visible = enabled && snapshot.key === nodeId ? snapshot : { ...IDLE_SNAPSHOT, loading: enabled };
  return {
    detail: visible.detail,
    failed: visible.failed,
    loading: visible.loading,
    retry: () => setAttempt((value) => value + 1),
  };
}
