// INPUT: Room 右栏类型、受控宽度百分比、窄屏标识与拖拽命令。
// OUTPUT: 统一封装分隔条、宽度计算与容器语义的 Room 右栏外壳。
// POS: 本地与在线 Room 共用的右栏几何容器；不持有业务面板状态。

import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { PanelResizeHandle } from "@/shared/ui/layout/panel-resize-handle";

import type { RoomSidePanelKind } from "./room-side-panel-width";
import { useRoomSidePanelResize } from "./use-room-side-panel-resize";

interface RoomSidePanelContainerProps {
  ariaLabel?: string;
  children: ReactNode;
  className?: string;
  compact?: boolean;
  kind: RoomSidePanelKind;
  onSidePanelWidthChange: (percent: number) => void;
  onStartSidePanelResize: () => void;
  sidePanelWidthPercent: number;
}

export function RoomSidePanelContainer({
  ariaLabel,
  children,
  className,
  compact = false,
  kind,
  onSidePanelWidthChange,
  onStartSidePanelResize,
  sidePanelWidthPercent,
}: RoomSidePanelContainerProps) {
  const { t } = useI18n();
  const resize = useRoomSidePanelResize(
    kind,
    sidePanelWidthPercent,
    onSidePanelWidthChange,
  );

  if (compact) {
    return <aside className="contents">{children}</aside>;
  }

  return (
    <>
      <PanelResizeHandle
        ariaLabel={t(
          kind === "thread"
            ? "room.resize_thread_panel"
            : "room.resize_auxiliary_panel",
        )}
        control={resize.control}
        controls={resize.panelId}
        onResizeStart={onStartSidePanelResize}
        variant="gutter"
      />

      <section
        aria-label={ariaLabel}
        id={resize.panelId}
        ref={resize.panelRef}
        className={cn(
          "nexus-room-surface-side-panel relative flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden",
          className,
        )}
        style={{
          width: `${sidePanelWidthPercent}%`,
          ...resize.widthStyle,
        }}
      >
        {children}
      </section>
    </>
  );
}
