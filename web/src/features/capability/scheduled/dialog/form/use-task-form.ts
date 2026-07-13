import { useCallback, useMemo, useState } from "react";

import type {
  ExecutionKind,
  ExecutionMode,
  ReplyMode,
  TargetType,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";

function clearContextSelection(
  current: TaskFormDraft,
  patch: Partial<TaskFormDraft>,
): TaskFormDraft {
  return {
    ...current,
    ...patch,
    selectedReplySessionKey: "",
    selectedSessionKey: "",
  };
}

export function useTaskForm(
  initialDraft: TaskFormDraft,
  onChange: () => void,
) {
  const [draft, setDraft] = useState(initialDraft);

  const hydrate = useCallback((nextDraft: TaskFormDraft) => {
    setDraft(nextDraft);
  }, []);

  const setValue = useCallback(<Key extends keyof TaskFormDraft>(
    key: Key,
    value: TaskFormDraft[Key],
  ) => {
    setDraft((current) => ({ ...current, [key]: value }));
    onChange();
  }, [onChange]);

  const setExecutionKind = useCallback((value: ExecutionKind) => {
    setDraft((current) => value === "script"
      ? clearContextSelection(current, {
          dedicatedSessionKey: "",
          executionKind: "script",
          executionMode: "temporary",
          replyMode: "none",
          targetType: "agent",
        })
      : { ...current, executionKind: "agent" });
    onChange();
  }, [onChange]);

  const setTargetType = useCallback((value: TargetType) => {
    setDraft((current) => {
      const targetType = current.executionKind === "script" ? "agent" : value;
      return clearContextSelection(current, {
        executionMode: targetType === "room" ? "existing" : current.executionMode,
        selectedRoomId: targetType === "room" ? current.selectedRoomId : "",
        targetType,
      });
    });
    onChange();
  }, [onChange]);

  const setExecutionMode = useCallback((value: ExecutionMode) => {
    setDraft((current) => {
      const executionMode = current.targetType === "room" ? "existing" : value;
      return {
        ...current,
        executionMode,
        replyMode: executionMode === "main" ? "none" : current.replyMode,
        selectedReplySessionKey: executionMode === "main"
          ? ""
          : current.selectedReplySessionKey,
        selectedSessionKey: executionMode === "existing"
          ? current.selectedSessionKey
          : "",
      };
    });
    onChange();
  }, [onChange]);

  const setSelectedAgentId = useCallback((value: string) => {
    setDraft((current) => clearContextSelection(current, {
      selectedAgentId: value,
    }));
    onChange();
  }, [onChange]);

  const setSelectedRoomId = useCallback((value: string) => {
    setDraft((current) => clearContextSelection(current, {
      selectedRoomId: value,
    }));
    onChange();
  }, [onChange]);

  const setReplyMode = useCallback((value: ReplyMode) => {
    setDraft((current) => {
      const replyMode = current.executionMode === "main" ? "none" : value;
      return {
        ...current,
        replyMode,
        selectedReplySessionKey: replyMode === "selected"
          ? current.selectedReplySessionKey
          : "",
      };
    });
    onChange();
  }, [onChange]);

  const actions = useMemo(() => ({
    setDedicatedSessionKey: (value: string) => setValue("dedicatedSessionKey", value),
    setEnabled: (value: boolean) => setValue("enabled", value),
    setExpiresAt: (value: string) => setValue("expiresAt", value),
    setExecutionKind,
    setExecutionMode,
    setInstruction: (value: string) => setValue("instruction", value),
    setReplyMode,
    setSelectedAgentId,
    setSelectedReplySessionKey: (value: string) => setValue(
      "selectedReplySessionKey",
      value,
    ),
    setSelectedRoomId,
    setSelectedSessionKey: (value: string) => setValue("selectedSessionKey", value),
    setTargetType,
    setTaskName: (value: string) => setValue("taskName", value),
  }), [
    setExecutionKind,
    setExecutionMode,
    setReplyMode,
    setSelectedAgentId,
    setSelectedRoomId,
    setTargetType,
    setValue,
  ]);

  return { actions, draft, hydrate };
}
