"use client";

import { useEffect } from "react";

import { GroupThreadContextProvider } from "../../group/thread/group-thread-context";
import { useGroupThread } from "../../group/thread/group-thread-state";
import { RoomSurfaceContent } from "./room-surface-content";
import type { RoomSurfaceLayoutProps } from "./room-surface-layout-types";

export function RoomSurfaceLayout(props: RoomSurfaceLayoutProps) {
  if (props.currentRoomType === "dm") {
    return <RoomSurfaceContent {...props} isThreadPanelOpen={false} />;
  }

  return (
    <GroupThreadContextProvider
      onOpenThread={() => props.onChangeSurfaceTab("chat")}
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
