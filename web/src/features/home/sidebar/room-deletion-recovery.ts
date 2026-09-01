/**
 * INPUT: Room 删除的结构化结果事实与权威目录核对状态。
 * OUTPUT: 是否允许再次 DELETE，以及确认框所需的本地化文案键。
 * POS: Home 侧栏 Room 删除的纯恢复模型；不得展示 FailureCore 内部字段。
 */
import type { MutationFailure } from "@/lib/error-message";
import type { TranslationKey } from "@/shared/i18n/messages";

export interface RoomDeletionFailure {
  directoryCheck: "failed" | "not_checked" | "target_absent" | "target_present";
  kind:
    | "committed_cleanup_incomplete"
    | "not_applied"
    | "outcome_unknown"
    | "resource_absent";
}

export type RoomDeletionCommand = "delete" | "dismiss" | "reconcile";

interface RoomDeletionRecoveryPresentation {
  confirmTextKey: TranslationKey;
  failure: {
    impactKey: TranslationKey;
    nextStepKey: TranslationKey;
    titleKey: TranslationKey;
  };
  variant: "danger" | "default";
}

type RoomDeletionRecoveryState =
  | "committed_confirmed"
  | "committed_failed"
  | "committed_pending"
  | "not_applied"
  | "resource_absent_conflict"
  | "resource_absent_failed"
  | "resource_absent_pending"
  | "unknown_failed"
  | "unknown_pending"
  | "unknown_present";

const ROOM_DELETION_RECOVERY_PRESENTATIONS: Record<
  RoomDeletionRecoveryState,
  RoomDeletionRecoveryPresentation
> = {
  committed_confirmed: roomDeletionPresentation("close", "committed_confirmed"),
  committed_failed: roomDeletionPresentation("check_list", "committed_failed"),
  committed_pending: roomDeletionPresentation("check_list", "committed_pending"),
  not_applied: roomDeletionPresentation("delete_again", "not_applied", "danger"),
  resource_absent_conflict: roomDeletionPresentation("check_list", "resource_absent_conflict"),
  resource_absent_failed: roomDeletionPresentation("check_list", "resource_absent_failed"),
  resource_absent_pending: roomDeletionPresentation("check_list", "resource_absent_pending"),
  unknown_failed: roomDeletionPresentation("check_list", "unknown_failed"),
  unknown_pending: roomDeletionPresentation("check_list", "unknown_pending"),
  unknown_present: roomDeletionPresentation("check_list", "unknown_present"),
};

export function projectRoomDeletionFailure(
  failure: Pick<MutationFailure, "code" | "effect">,
): RoomDeletionFailure {
  const kind: RoomDeletionFailure["kind"] = failure.code === "room.not_found"
    ? "resource_absent"
    : failure.effect === "committed"
      ? "committed_cleanup_incomplete"
      : failure.effect === "not_applied"
        ? "not_applied"
        : "outcome_unknown";
  return { directoryCheck: "not_checked", kind };
}

export function getRoomDeletionCommand(
  failure: RoomDeletionFailure | null,
): RoomDeletionCommand {
  if (failure === null || failure.kind === "not_applied") {
    return "delete";
  }
  return failure.kind === "committed_cleanup_incomplete"
    && failure.directoryCheck === "target_absent"
    ? "dismiss"
    : "reconcile";
}

export function getRoomDeletionRecoveryPresentation(
  failure: RoomDeletionFailure,
): RoomDeletionRecoveryPresentation {
  return ROOM_DELETION_RECOVERY_PRESENTATIONS[roomDeletionRecoveryState(failure)];
}

function roomDeletionRecoveryState(
  failure: RoomDeletionFailure,
): RoomDeletionRecoveryState {
  if (failure.kind === "not_applied") {
    return "not_applied";
  }
  if (failure.kind === "committed_cleanup_incomplete") {
    return failure.directoryCheck === "failed"
      ? "committed_failed"
      : failure.directoryCheck === "target_absent"
        ? "committed_confirmed"
        : "committed_pending";
  }
  if (failure.kind === "resource_absent") {
    return failure.directoryCheck === "failed"
      ? "resource_absent_failed"
      : failure.directoryCheck === "target_present"
        ? "resource_absent_conflict"
        : "resource_absent_pending";
  }
  return failure.directoryCheck === "failed"
    ? "unknown_failed"
    : failure.directoryCheck === "target_present"
      ? "unknown_present"
      : "unknown_pending";
}

function roomDeletionPresentation(
  confirm: "check_list" | "close" | "delete_again",
  state: RoomDeletionRecoveryState,
  variant: RoomDeletionRecoveryPresentation["variant"] = "default",
): RoomDeletionRecoveryPresentation {
  const prefix = `home.room_delete_recovery.${state}` as const;
  return {
    confirmTextKey: `home.room_delete_recovery.action_${confirm}`,
    failure: {
      impactKey: `${prefix}.impact`,
      nextStepKey: `${prefix}.next_step`,
      titleKey: `${prefix}.title`,
    },
    variant,
  };
}
