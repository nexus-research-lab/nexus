/**
 * INPUT: Room 删除的结构化结果事实、权威目录核对状态与当前界面语言。
 * OUTPUT: 是否允许再次 DELETE，以及回答结果、影响、下一步的确认框展示。
 * POS: Home 侧栏 Room 删除的纯恢复模型；不得展示 FailureCore 内部字段。
 */
import type { MutationFailure } from "@/lib/error-message";
import type { Locale } from "@/shared/i18n/messages";

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
  confirmText: string;
  failure: {
    impact: string;
    nextStep: string;
    title: string;
  };
  variant: "danger" | "default";
}

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
  locale: Locale,
): RoomDeletionRecoveryPresentation {
  return locale === "en"
    ? getEnglishPresentation(failure)
    : getChinesePresentation(failure);
}

function getChinesePresentation(
  failure: RoomDeletionFailure,
): RoomDeletionRecoveryPresentation {
  if (failure.kind === "not_applied") {
    return {
      confirmText: "再次删除",
      failure: {
        title: "Room 没有删除",
        impact: "这次删除没有生效；Room、已有会话和内容仍然保留。",
        nextStep: "可以再次删除，或取消后继续保留。",
      },
      variant: "danger",
    };
  }
  if (failure.kind === "committed_cleanup_incomplete") {
    const directoryConfirmed = failure.directoryCheck === "target_absent";
    return {
      confirmText: directoryConfirmed ? "关闭" : "核对 Room 列表",
      failure: failure.directoryCheck === "failed"
        ? {
            title: "Room 已删除，列表暂时无法更新",
            impact: "删除已经提交，但部分关联内容没有完全清理；当前列表仍显示上次内容。",
            nextStep: "检查网络连接后重新核对 Room 列表，不要再次删除。",
          }
        : directoryConfirmed
          ? {
              title: "Room 已删除，关联内容未完全清理",
              impact: "权威列表已确认 Room 不再存在；部分会话、工作内容或运行状态仍需处理。",
              nextStep: "可以关闭此提示，不要再次删除。",
            }
          : {
            title: "Room 已删除，关联内容未完全清理",
            impact: "删除已经提交；部分会话、工作内容或运行状态仍需处理。",
            nextStep: "核对 Room 列表确认最新状态，不要再次删除。",
          },
      variant: "default",
    };
  }
  if (failure.kind === "resource_absent") {
    return {
      confirmText: "核对 Room 列表",
      failure: failure.directoryCheck === "failed"
        ? {
            title: "Room 列表暂时无法更新",
            impact: "服务端已确认这个 Room 不存在，但当前列表仍是上次内容；这次请求没有删除其他 Room。",
            nextStep: "检查网络连接后重新核对 Room 列表。",
          }
        : failure.directoryCheck === "target_present"
          ? {
              title: "Room 仍在列表中，当前状态存在冲突",
              impact: "删除接口没有找到 Room，但权威列表仍包含它；当前无法证明已有会话和内容未受影响。",
              nextStep: "稍后再次核对 Room 列表；确认前不要重新删除。",
            }
          : {
            title: "Room 已经不存在",
            impact: "服务端没有找到这个 Room；这次请求没有删除其他 Room，当前列表尚未核对。",
            nextStep: "正在核对 Room 列表，不要再次删除。",
          },
      variant: "default",
    };
  }
  return {
    confirmText: "核对 Room 列表",
    failure: failure.directoryCheck === "failed"
      ? {
          title: "Room 状态仍无法确认",
          impact: "删除结果未确认，当前列表显示上次内容；重复删除有重复清理风险。",
          nextStep: "检查网络连接后重新核对 Room 列表，不要再次删除。",
        }
      : failure.directoryCheck === "target_present"
        ? {
            title: "Room 仍在列表中，但删除过程未完全确认",
            impact: "当前列表仍有 Room，但一次读取不能证明先前删除结果；确认前保持保护。",
            nextStep: "稍后再次核对 Room 列表；确认前不要重新删除。",
          }
        : {
          title: "还无法确认 Room 是否已删除",
          impact: "未收到完整结果；Room 删除结果待核对，系统没有自动再次删除。",
          nextStep: "正在核对 Room 列表；确认状态前不要再次删除。",
        },
    variant: "default",
  };
}

function getEnglishPresentation(
  failure: RoomDeletionFailure,
): RoomDeletionRecoveryPresentation {
  if (failure.kind === "not_applied") {
    return {
      confirmText: "Delete again",
      failure: {
        title: "The Room wasn’t deleted",
        impact: "This deletion did not take effect. The Room, its conversations, and existing content remain.",
        nextStep: "You can delete it again, or cancel to keep it.",
      },
      variant: "danger",
    };
  }
  if (failure.kind === "committed_cleanup_incomplete") {
    const directoryConfirmed = failure.directoryCheck === "target_absent";
    return {
      confirmText: directoryConfirmed ? "Close" : "Check Room list",
      failure: failure.directoryCheck === "failed"
        ? {
            title: "The Room was deleted, but the list can’t be updated",
            impact: "The deletion was committed, but some related content was not fully cleaned up. The list shows its previous state.",
            nextStep: "Check your connection, then check the Room list again. Don’t delete it again.",
          }
        : directoryConfirmed
          ? {
              title: "The Room was deleted, but cleanup is incomplete",
              impact: "The authoritative list confirms that the Room no longer exists. Some conversations, work, or running state still need attention.",
              nextStep: "You can close this message. Don’t delete the Room again.",
            }
          : {
            title: "The Room was deleted, but cleanup is incomplete",
            impact: "The deletion was committed. Some conversations, work, or running state still need attention.",
            nextStep: "Check the Room list for the latest state. Don’t delete it again.",
          },
      variant: "default",
    };
  }
  if (failure.kind === "resource_absent") {
    return {
      confirmText: "Check Room list",
      failure: failure.directoryCheck === "failed"
        ? {
            title: "The Room list can’t be updated",
            impact: "The server confirmed that this Room does not exist, but the list still shows its previous state. No other Room was deleted by this request.",
            nextStep: "Check your connection, then check the Room list again.",
          }
        : failure.directoryCheck === "target_present"
          ? {
              title: "The Room is still listed, so its state conflicts",
              impact: "The delete endpoint could not find the Room, but the authoritative list still includes it. Nexus can’t prove that existing conversations and content were unaffected.",
              nextStep: "Check the Room list again later. Don’t delete it again until the result is confirmed.",
            }
          : {
            title: "The Room no longer exists",
            impact: "The server could not find this Room. No other Room was deleted, and the list has not yet been checked.",
            nextStep: "Nexus is checking the Room list. Don’t delete it again.",
          },
      variant: "default",
    };
  }
  return {
    confirmText: "Check Room list",
    failure: failure.directoryCheck === "failed"
      ? {
          title: "The Room’s status still can’t be confirmed",
          impact: "The deletion result is unconfirmed and the list shows its previous state. Deleting again carries a duplicate-cleanup risk.",
          nextStep: "Check your connection, then check the Room list again. Don’t delete it again.",
        }
      : failure.directoryCheck === "target_present"
        ? {
            title: "The Room is still listed, but the deletion is not fully confirmed",
            impact: "The Room is still listed, but one read cannot prove the earlier deletion result. Protection remains until it is confirmed.",
            nextStep: "Check the Room list again later. Don’t delete it again until the result is confirmed.",
          }
        : {
          title: "We can’t confirm whether the Room was deleted",
          impact: "Nexus did not receive a complete result. The Room deletion result needs verification, and Nexus did not delete it again.",
          nextStep: "Nexus is checking the Room list. Don’t delete it again until its state is confirmed.",
        },
    variant: "default",
  };
}
