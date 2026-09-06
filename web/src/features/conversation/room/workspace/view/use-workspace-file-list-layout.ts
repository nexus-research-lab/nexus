// INPUT: 当前紧凑断点与外部是否允许调整目录宽度。
// OUTPUT: 有界目录宽度、键盘调整范围和共享鼠标拖动生命周期。
// POS: Workspace 文件列表尺寸 owner；不处理文件导航或写入。

import { useCallback, useEffect, useRef, useState } from "react";

import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { useMouseDrag } from "@/shared/lib/react/use-mouse-drag";

interface FileListLayoutSpec {
  defaultWidth: number;
  minWidth: number;
  maxWidth: number;
}

const FILE_LIST_LAYOUT_BY_MODE: Record<"regular" | "compact", FileListLayoutSpec> = {
  regular: {defaultWidth: 280, minWidth: 200, maxWidth: 360},
  compact: {defaultWidth: 220, minWidth: 160, maxWidth: 280},
};

function clampWidth(width: number, spec: FileListLayoutSpec): number {
  return Math.min(Math.max(width, spec.minWidth), spec.maxWidth);
}

export function useWorkspaceFileListLayout(enabled = true) {
  const panelRef = useRef<HTMLDivElement>(null);
  const isCompact = useMediaQuery("(max-width: 1280px)");
  const mode = isCompact ? "compact" : "regular";
  const spec = FILE_LIST_LAYOUT_BY_MODE[mode];
  const [width, setWidth] = useState(FILE_LIST_LAYOUT_BY_MODE.regular.defaultWidth);

  useEffect(() => {
    setWidth((current) => (
      isCompact
        ? Math.min(current, spec.defaultWidth)
        : Math.max(current, spec.defaultWidth)
    ));
  }, [isCompact, spec.defaultWidth]);

  const handleMove = useCallback((event: MouseEvent) => {
    const bounds = panelRef.current?.getBoundingClientRect();
    if (bounds && bounds.width > 0) {
      setWidth(clampWidth(bounds.right - event.clientX, spec));
    }
  }, [spec]);
  const { isDragging, startDragging, stopDragging } = useMouseDrag(handleMove, enabled);
  const changeWidth = useCallback((next: number) => {
    if (enabled && Number.isFinite(next)) setWidth(clampWidth(next, spec));
  }, [enabled, spec]);
  const visibleWidth = clampWidth(width, spec);

  return {
    panelRef,
    width: visibleWidth,
    resizeControl: enabled ? { value: visibleWidth, min: spec.minWidth, max: spec.maxWidth, onChange: changeWidth } : null,
    isResizing: isDragging,
    startResizing: startDragging,
    stopResizing: stopDragging,
  };
}
