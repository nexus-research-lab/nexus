import { useCallback, useMemo, useState } from "react";

import type {
  DeliveryTargetType,
  ExecutionMode,
  PermissionMode,
  ReplyMode,
  TargetType,
  TaskFormDraft,
  TaskDestinationOption,
  TaskDialogSessionOption,
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
        permissionMode: targetType !== current.targetType ? "copy" : current.permissionMode,
        selectedAgentId: targetType === "room" ? "" : current.selectedAgentId,
        selectedRoomId: targetType === "room" ? current.selectedRoomId : "",
        targetType,
        deliveryTargetType: targetType,
        selectedReplySessionKey: "",
        selectedDeliveryRoomId: "",
        selectedDeliveryPresenterAgentId: "",
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

  // 只选择唯一的合法候选；不覆盖用户选择，不猜测多个聊天的用途。
  const resolveDefaultSessions = useCallback((execution: TaskDialogSessionOption[], delivery: TaskDialogSessionOption[]) => {
    setDraft((current) => {
      const selectedSessionKey = current.selectedSessionKey
        || (current.executionMode === "existing" && execution.length === 1 && !execution[0].disabled ? execution[0].value : "");
      const sameTarget = current.targetType === current.deliveryTargetType && (current.targetType === "room"
        ? current.selectedRoomId === current.selectedDeliveryRoomId
        : current.selectedAgentId === current.selectedDeliveryAgentId);
      const matching = sameTarget && delivery.find((option) => option.value === selectedSessionKey && !option.disabled);
      const selectedReplySessionKey = current.selectedReplySessionKey || (current.replyMode === "selected"
        ? matching ? matching.value : delivery.length === 1 && !delivery[0].disabled ? delivery[0].value : ""
        : "");
      return selectedSessionKey === current.selectedSessionKey && selectedReplySessionKey === current.selectedReplySessionKey
        ? current : { ...current, selectedSessionKey, selectedReplySessionKey };
    });
  }, []);

  const setSelectedAgentId = useCallback((value: string) => {
    setDraft((current) => {
      const patch = {
        permissionMode: value.trim() !== current.selectedAgentId.trim()
          ? "copy" as const
          : current.permissionMode,
        selectedAgentId: value,
        ...(current.targetType === "agent" && current.deliveryTargetType === "agent" && current.selectedDeliveryAgentId === current.selectedAgentId
          ? { selectedDeliveryAgentId: value, selectedReplySessionKey: "" } : {}),
      };
      return current.targetType === "room"
        ? { ...current, ...patch }
        : clearExecutionSelection(current, patch);
    });
    onChange();
  }, [onChange]);

  const setSelectedRoomId = useCallback((value: string) => {
    setDraft((current) => clearExecutionSelection(current, {
      permissionMode: value.trim() !== current.selectedRoomId.trim()
        ? "copy"
        : current.permissionMode,
      selectedAgentId: "",
      selectedRoomId: value,
      ...(current.deliveryTargetType === "room" && current.selectedDeliveryRoomId === current.selectedRoomId
        ? { selectedDeliveryRoomId: value, selectedReplySessionKey: "", selectedDeliveryPresenterAgentId: "" } : {}),
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
        selectedDeliveryPresenterAgentId: replyMode === "selected"
          ? current.selectedDeliveryPresenterAgentId
          : "",
      };
    });
    onChange();
  }, [onChange]);

  const setDeliveryTargetType = useCallback((value: DeliveryTargetType) => {
    setDraft((current) => ({
      ...current,
      deliveryTargetType: value,
      selectedDeliveryPresenterAgentId: "",
      selectedReplySessionKey: "",
    }));
    onChange();
  }, [onChange]);

  const setSelectedDeliveryAgentId = useCallback((value: string) => {
    setDraft((current) => ({
      ...current,
      selectedDeliveryAgentId: value,
      selectedDeliveryPresenterAgentId: "",
      selectedReplySessionKey: "",
    }));
    onChange();
  }, [onChange]);

  const setSelectedDeliveryRoomId = useCallback((value: string) => {
    setDraft((current) => ({
      ...current,
      selectedDeliveryRoomId: value,
      selectedDeliveryPresenterAgentId: "",
      selectedReplySessionKey: "",
    }));
    onChange();
  }, [onChange]);

  const setSelectedSessionKey = useCallback((value: string) => {
    setDraft((current) => ({
      ...current,
      permissionMode: current.targetType === "room" && current.selectedAgentId
        ? "copy"
        : current.permissionMode,
      selectedAgentId: current.targetType === "room" ? "" : current.selectedAgentId,
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

  const selectExecution = useCallback((option: TaskDestinationOption) => {
    setDraft((current) => ({
      ...current,
      targetType: option.targetType,
      selectedAgentId: option.agentId,
      selectedRoomId: option.roomId,
      selectedSessionKey: option.sessionKey,
      executionMode: option.sessionKey ? "existing" : "temporary",
      dedicatedSessionKey: "",
      permissionMode: "copy",
    }));
    onChange();
  }, [onChange]);

  const selectDelivery = useCallback((option: TaskDestinationOption | null) => {
    setDraft((current) => ({
      ...current,
      replyMode: option && current.executionMode !== "main" ? "selected" : "none",
      deliveryTargetType: option?.targetType ?? "agent",
      selectedDeliveryAgentId: option?.agentId ?? "",
      selectedDeliveryRoomId: option?.roomId ?? "",
      selectedReplySessionKey: current.executionMode === "main" ? "" : option?.sessionKey ?? "",
      selectedDeliveryPresenterAgentId: "",
    }));
    onChange();
  }, [onChange]);

  const actions = useMemo(() => ({
    selectExecution,
    selectDelivery,
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
    setSelectedDeliveryPresenterAgentId: (value: string) => setValue(
      "selectedDeliveryPresenterAgentId",
      value,
    ),
    setSelectedReplySessionKey: (value: string) => {
      setDraft((current) => ({
        ...current,
        selectedDeliveryPresenterAgentId: "",
        selectedReplySessionKey: value,
      }));
      onChange();
    },
    setSelectedRoomId,
    setSelectedSessionKey,
    setTargetType,
    setTaskName: (value: string) => setValue("taskName", value),
  }), [
    selectExecution,
    selectDelivery,
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
    onChange,
  ]);

  return { actions, draft, hydrate, resolveDefaultSessions };
}
