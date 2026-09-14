// INPUT: DM/Room 布局类型、页面命令和精确 Thread 选择。
// OUTPUT: 稳定 Thread 控制上下文与互斥辅助页，主聊天沿原位置挂载。
// POS: 桌面 Room 布局入口；实时数据归 live owner，页面快照不重建控制回调。
"use client";

import { useCallback, useEffect } from "react";

import { GroupThreadContextProvider } from "../../group/thread/group-thread-context";
import { useGroupThread } from "../../group/thread/group-thread-state";
import { RoomSurfaceContent } from "./room-surface-content";
import type { RoomSurfaceLayoutProps } from "./room-surface-layout-types";

export function RoomSurfaceLayout(props: RoomSurfaceLayoutProps) {
  const { onChangeSurfaceTab } = props;
  const handleOpenThread = useCallback(() => onChangeSurfaceTab("chat"), [onChangeSurfaceTab]);
  if (props.currentRoomType === "dm") {
    return <RoomSurfaceContent {...props} isThreadPanelOpen={false} />;
  }

  return (
    <GroupThreadContextProvider
      onOpenThread={handleOpenThread}
    >
      <GroupRoomSurfaceLayout {...props} />
    </GroupThreadContextProvider>
  );
}

function GroupRoomSurfaceLayout(props: RoomSurfaceLayoutProps) {
  // 祖先只订阅稳定的控制状态，Thread 数据由兄弟叶子自行读取，避免反馈渲染。
  const { activeThread, closeThread } = useGroupThread();

  useEffect(() => {
    if (props.activeSurfaceTab !== "chat" && activeThread) {
      closeThread();
    }
  }, [activeThread, closeThread, props.activeSurfaceTab]);

  return (
    <RoomSurfaceContent
      {...props}
      isThreadPanelOpen={Boolean(activeThread)}
    />
  );
}
