"use client";

/**
 * INPUT: exact task source/id, server capabilities and transcript refresh.
 * OUTPUT: scope-fenced stop/send mutations、重复抑制与保守的 FailureCore 写入结果事实。
 * POS: 子智能体线程动作控制器；旧响应和传输失败保持 unknown，视图不构造路由或猜测 runtime 能力。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import {
  sendSubagentTaskMessageApi,
  stopSubagentTaskApi,
} from "@/lib/api/conversation/subagent-task-api";
import {
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  SubagentTask,
  SubagentTaskSource,
} from "@/types/conversation/subagent-task";

import type { TranslationKey } from "@/shared/i18n/messages";

import {
  canSendSubagentTaskMessage,
  isSubagentTaskActive,
  subagentTaskSourceKey,
} from "../subagent-task-model";

export type SubagentTaskPendingAction = "send" | "stop" | null;

export interface SubagentTaskActionFailure {
  action: Exclude<SubagentTaskPendingAction, null>;
  effect: MutationFailureEffect;
  message: string;
}

export interface SubagentTaskActions {
  error: SubagentTaskActionFailure | null;
  feedback: TranslationKey | null;
  pendingAction: SubagentTaskPendingAction;
  send: (message: string) => Promise<boolean>;
  stop: () => Promise<boolean>;
}

export interface SubagentTaskActionRequest {
  action: Exclude<SubagentTaskPendingAction, null>;
  request: (signal: AbortSignal) => Promise<void>;
  successMessage: TranslationKey;
}

export function useSubagentTaskActions({
  refresh,
  source,
  task,
}: {
  refresh: (silent?: boolean) => Promise<void>;
  source: SubagentTaskSource;
  task: SubagentTask;
}): SubagentTaskActions {
  const { t } = useI18n();
  const scopeKey = `${subagentTaskSourceKey(source)}:${task.task_id}`;
  const generationRef = useRef(0);
  const controllerRef = useRef<AbortController | null>(null);
  const [error, setError] = useState<SubagentTaskActionFailure | null>(null);
  const [feedback, setFeedback] = useState<TranslationKey | null>(null);
  const [pendingAction, setPendingAction] = useState<SubagentTaskPendingAction>(null);

  useEffect(() => {
    generationRef.current += 1;
    controllerRef.current?.abort();
    controllerRef.current = null;
    setError(null);
    setFeedback(null);
    setPendingAction(null);
    return () => {
      generationRef.current += 1;
      controllerRef.current?.abort();
    };
  }, [scopeKey]);

  useEffect(() => {
    if (isSubagentTaskActive(task)) return;
    setError((current) => current?.action === "stop" ? null : current);
  }, [task]);

  const run = useCallback(async ({
    action,
    request,
    successMessage,
  }: SubagentTaskActionRequest): Promise<boolean> => {
    // React state updates are not synchronous. The controller ref closes the
    // same-render double-click window before pendingAction is rendered.
    if (pendingAction !== null || controllerRef.current !== null) {
      return false;
    }
    const generation = generationRef.current;
    const controller = new AbortController();
    controllerRef.current = controller;
    setPendingAction(action);
    setError(null);
    setFeedback(null);
    try {
      await request(controller.signal);
      if (generation !== generationRef.current || controller.signal.aborted) {
        return false;
      }
      setFeedback(successMessage);
      // The mutation has already succeeded. A transcript refresh is a
      // separate best-effort read and must not turn that receipt into an error.
      await refresh(false).catch(() => undefined);
      return true;
    } catch (requestError) {
      if (generation !== generationRef.current || controller.signal.aborted) {
        return false;
      }
      const failure = projectMutationFailure(
        requestError,
        t(action === "stop"
          ? "subagents.stop_failed_detail"
          : "subagents.message_failed_detail"),
      );
      setError({
        action,
        effect: failure.effect,
        message: failure.message,
      });
      return false;
    } finally {
      if (generation === generationRef.current && controllerRef.current === controller) {
        controllerRef.current = null;
        setPendingAction(null);
      }
    }
  }, [pendingAction, refresh, t]);

  const stop = useCallback(() => {
    if (!isSubagentTaskActive(task) || !task.capabilities.stop) {
      return Promise.resolve(false);
    }
    return run({
      action: "stop",
      request: async (signal) => {
        await stopSubagentTaskApi(source, task.task_id, signal);
      },
      successMessage: "subagents.stop_succeeded",
    });
  }, [run, source, task]);

  const send = useCallback((message: string) => {
    const normalized = message.trim();
    if (!normalized || !canSendSubagentTaskMessage(task)) {
      return Promise.resolve(false);
    }
    return run({
      action: "send",
      request: async (signal) => {
        await sendSubagentTaskMessageApi(source, task.task_id, normalized, signal);
      },
      successMessage: "subagents.message_queued",
    });
  }, [run, source, task]);

  return { error, feedback, pendingAction, send, stop };
}
