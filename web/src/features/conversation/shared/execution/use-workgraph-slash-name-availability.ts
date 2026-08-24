/**
 * INPUT: 当前 canonical Slash 候选、exact preview_id 与本地格式校验结果。
 * OUTPUT: 与当前输入绑定、可取消且防抖的 owner-scoped 名称可用性状态。
 * POS: 保存确认页的轻量异步校验控制器；最终唯一性仍由服务端保存事务裁决。
 */
"use client";

import { useEffect, useState } from "react";

import { checkWorkGraphWorkflowSlashNameApi } from "@/lib/api/conversation/execution-api";

export type WorkGraphSlashNameAvailabilityStatus =
  | "idle"
  | "checking"
  | "available"
  | "unavailable"
  | "error";

export interface WorkGraphSlashNameAvailabilityState {
  slashName: string;
  status: WorkGraphSlashNameAvailabilityStatus;
}

const SLASH_NAME_CHECK_DELAY_MS = 350;

export function useWorkGraphSlashNameAvailability({
  enabled,
  previewId,
  slashName,
}: {
  enabled: boolean;
  previewId: string;
  slashName: string;
}): WorkGraphSlashNameAvailabilityState {
  const [state, setState] = useState<WorkGraphSlashNameAvailabilityState>(() => ({
    slashName,
    status: enabled ? "checking" : "idle",
  }));

  useEffect(() => {
    if (!enabled) {
      setState({ slashName, status: "idle" });
      return;
    }
    const controller = new AbortController();
    setState({ slashName, status: "checking" });
    const timeout = window.setTimeout(() => {
      void checkWorkGraphWorkflowSlashNameApi(
        slashName,
        previewId,
        controller.signal,
      ).then((result) => {
        if (!controller.signal.aborted) {
          setState({
            slashName: result.slash_name,
            status: result.available ? "available" : "unavailable",
          });
        }
      }).catch(() => {
        if (!controller.signal.aborted) {
          setState({ slashName, status: "error" });
        }
      });
    }, SLASH_NAME_CHECK_DELAY_MS);
    return () => {
      window.clearTimeout(timeout);
      controller.abort();
    };
  }, [enabled, previewId, slashName]);

  return state;
}
