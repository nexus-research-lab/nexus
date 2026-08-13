import { useCallback, useMemo, useState } from "react";

import type {
  DeliveryTargetType,
  ExecutionMode,
  PermissionMode,
  ReplyMode,
  TargetType,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";

function clearExecutionSelection(
  current: TaskFormDraft,
  patch: Partial<TaskFormDraft>,
): TaskFormDraft {
  return {
    ...current,
    ...patch,
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

  const setTargetType = useCallback((value: TargetType) => {
    setDraft((current) => {
      const targetType = current.executionKind === "script" ? "agent" : value;
      return clearExecutionSelection(current, {
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
    setDraft((current) => clearExecutionSelection(current, {
      permissionMode: value.trim() !== current.selectedAgentId.trim()
        ? "copy"
        : current.permissionMode,
      selectedAgentId: value,
    }));
    onChange();
  }, [onChange]);

  const setSelectedRoomId = useCallback((value: string) => {
    setDraft((current) => clearExecutionSelection(current, {
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

  const setDeliveryTargetType = useCallback((value: DeliveryTargetType) => {
    setDraft((current) => ({
      ...current,
      deliveryTargetType: value,
      selectedReplySessionKey: "",
    }));
    onChange();
  }, [onChange]);

  const setSelectedDeliveryAgentId = useCallback((value: string) => {
    setDraft((current) => ({
      ...current,
      selectedDeliveryAgentId: value,
      selectedReplySessionKey: "",
    }));
    onChange();
  }, [onChange]);

  const setSelectedDeliveryRoomId = useCallback((value: string) => {
    setDraft((current) => ({
      ...current,
      selectedDeliveryRoomId: value,
      selectedReplySessionKey: "",
    }));
    onChange();
  }, [onChange]);

  const setSelectedSessionKey = useCallback((value: string, agentId?: string) => {
    const normalizedAgentId = agentId?.trim() || "";
    setDraft((current) => ({
      ...current,
      permissionMode: normalizedAgentId
        && normalizedAgentId !== current.selectedAgentId.trim()
        ? "copy"
        : current.permissionMode,
      selectedAgentId: normalizedAgentId || current.selectedAgentId,
      selectedSessionKey: value,
    }));
    onChange();
  }, [onChange]);

  const resolveSelectedRoomIds = useCallback((values: {
    deliveryRoomId?: string;
    executionRoomId?: string;
  }) => {
    setDraft((current) => {
      const selectedDeliveryRoomId = current.selectedDeliveryRoomId
        || values.deliveryRoomId?.trim()
        || "";
      const selectedRoomId = current.selectedRoomId
        || values.executionRoomId?.trim()
        || "";
      if (selectedDeliveryRoomId === current.selectedDeliveryRoomId
        && selectedRoomId === current.selectedRoomId) {
        return current;
      }
      return { ...current, selectedDeliveryRoomId, selectedRoomId };
    });
  }, []);

  const actions = useMemo(() => ({
    setDedicatedSessionKey: (value: string) => setValue("dedicatedSessionKey", value),
    setDeliveryTargetType,
    setEnabled: (value: boolean) => setValue("enabled", value),
    setExpiresAt: (value: string) => setValue("expiresAt", value),
    setExecutionMode,
    setInstruction: (value: string) => setValue("instruction", value),
    setPermissionMode: (value: PermissionMode) => setValue("permissionMode", value),
    setReplyMode,
    resolveSelectedRoomIds,
    setSelectedAgentId,
    setSelectedDeliveryAgentId,
    setSelectedDeliveryRoomId,
    setSelectedReplySessionKey: (value: string) => setValue(
      "selectedReplySessionKey",
      value,
    ),
    setSelectedRoomId,
    setSelectedSessionKey,
    setTargetType,
    setTaskName: (value: string) => setValue("taskName", value),
  }), [
    resolveSelectedRoomIds,
    setExecutionMode,
    setDeliveryTargetType,
    setReplyMode,
    setSelectedAgentId,
    setSelectedDeliveryAgentId,
    setSelectedDeliveryRoomId,
    setSelectedRoomId,
    setSelectedSessionKey,
    setTargetType,
    setValue,
  ]);

  return { actions, draft, hydrate };
}
