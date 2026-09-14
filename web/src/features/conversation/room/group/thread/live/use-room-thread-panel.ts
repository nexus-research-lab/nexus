// INPUT: 当前精确 Thread、Room live source 和界面语言。
// OUTPUT: 随 source/目标/语言更新的只读 Thread 面板模型。
// POS: live store 到模型的 React 适配，不创建或改写业务身份。

import { useMemo } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";

import { useGroupThread } from "../group-thread-state";
import { useRoomThreadLiveStore } from "./room-thread-live-store";
import {
  buildRoomThreadPanelModel,
  type RoomThreadPanelModel,
} from "./room-thread-panel-model";

export function useRoomThreadPanel(): RoomThreadPanelModel | null {
  const { t } = useI18n();
  const { activeThread } = useGroupThread();
  const source = useRoomThreadLiveStore((state) => state.source);
  return useMemo(
    () => buildRoomThreadPanelModel(source, activeThread, t),
    [activeThread, source, t],
  );
}
