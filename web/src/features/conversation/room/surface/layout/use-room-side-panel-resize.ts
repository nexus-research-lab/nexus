// INPUT: 右栏类型、受控百分比与原布局 owner 的宽度更新入口。
// OUTPUT: 基于实际百分比容器的像素调整范围、面板关联和唯一 CSS 限制。
// POS: Room 布局几何适配；键盘像素请求转换回原百分比，不拥有第二份宽度偏好。

import { useId, useLayoutEffect, useState } from "react";

import type { PanelResizeControl } from "@/shared/ui/layout/panel-resize-handle";

import { getRoomSidePanelWidthRange, getRoomSidePanelWidthStyle, type RoomSidePanelKind } from "./room-side-panel-width";

export function useRoomSidePanelResize(kind: RoomSidePanelKind, widthPercent: number, onWidthChange: (percent: number) => void) {
  const panelId = useId();
  const [panel, setPanel] = useState<HTMLElement | null>(null);
  const [geometry, setGeometry] = useState({ containerWidth: 0, viewportWidth: 0 });
  useLayoutEffect(() => {
    const container = panel?.parentElement;
    if (!container) return;
    let disposed = false;
    const update = () => {
      if (disposed) return;
      const containerWidth = container.getBoundingClientRect().width;
      const viewportWidth = window.innerWidth;
      setGeometry((current) => current.containerWidth === containerWidth && current.viewportWidth === viewportWidth
        ? current : { containerWidth, viewportWidth });
    };
    update();
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(update);
    observer?.observe(container);
    window.addEventListener("resize", update);
    return () => {
      disposed = true;
      observer?.disconnect();
      window.removeEventListener("resize", update);
    };
  }, [panel]);
  const range = panel ? getRoomSidePanelWidthRange(kind, widthPercent, geometry.containerWidth, geometry.viewportWidth) : null;
  const control: PanelResizeControl | null = range ? {
    ...range,
    onChange: (width) => onWidthChange(width / geometry.containerWidth * 100),
  } : null;
  return { panelId, panelRef: setPanel, control, widthStyle: getRoomSidePanelWidthStyle(kind) };
}
