// INPUT: 领域已筛选的 Room/Session 候选、Room 显示目录与当前语言。
// OUTPUT: 不暴露内部 ID、同名可区分且保留原路由和徽标的选项。
// POS: Automation 选择文字适配；只消费 shared/lib 序号算法，不增加候选或修改绑定。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { buildNamedSelectionOptions } from "@/shared/lib/selection-options";
import type { RoomRecord } from "@/types/conversation/room";
import type { TaskDialogSessionOption } from "../scheduled-task-dialog-types";

export function buildRoomSelectionOptions(
  rooms: readonly RoomRecord[],
  t: I18nContextValue["t"],
  directory: readonly RoomRecord[] = rooms,
) {
  const source = (room: RoomRecord) => {
    const createdAt = Date.parse(room.created_at || "");
    return { value: room.id, label: room.name, createdAt: Number.isFinite(createdAt) ? createdAt : undefined };
  };
  return buildNamedSelectionOptions(rooms.map(source), {
    fallbackLabel: t("room.untitled_collaboration"),
    numberedLabel: (name, number) => t("common.selection_numbered", { name, number }),
  }, directory.map(source));
}

export function distinguishSessionOptions(options: TaskDialogSessionOption[], t: I18nContextValue["t"]): TaskDialogSessionOption[] {
  const labels = buildNamedSelectionOptions(options, {
    fallbackLabel: t("capability.scheduled_dialog_unnamed_session"),
    numberedLabel: (name, number) => t("common.selection_numbered", { name, number }),
  });
  return options.map((option, index) => ({ ...option, label: labels[index].label }));
}
