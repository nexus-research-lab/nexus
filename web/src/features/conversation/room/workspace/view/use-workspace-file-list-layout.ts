import { useCallback, useEffect, useRef, useState } from "react";

import { useMediaQuery } from "@/hooks/ui/use-media-query";

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

export function useWorkspaceFileListLayout() {
  const panelRef = useRef<HTMLDivElement>(null);
  const isCompact = useMediaQuery("(max-width: 1280px)");
  const mode = isCompact ? "compact" : "regular";
  const spec = FILE_LIST_LAYOUT_BY_MODE[mode];
  const [width, setWidth] = useState(FILE_LIST_LAYOUT_BY_MODE.regular.defaultWidth);
  const [isResizing, setIsResizing] = useState(false);

  useEffect(() => {
    setWidth((current) => (
      isCompact
        ? Math.min(current, spec.defaultWidth)
        : Math.max(current, spec.defaultWidth)
    ));
  }, [isCompact, spec.defaultWidth]);

  useEffect(() => {
    if (!isResizing) {
      return;
    }

    const handleMouseMove = (event: MouseEvent) => {
      const bounds = panelRef.current?.getBoundingClientRect();
      if (bounds) {
        setWidth(clampWidth(bounds.right - event.clientX, spec));
      }
    };
    const handleMouseUp = () => setIsResizing(false);

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizing, spec]);

  const startResizing = useCallback(() => setIsResizing(true), []);
  const stopResizing = useCallback(() => setIsResizing(false), []);

  return {
    panelRef,
    width,
    isResizing,
    startResizing,
    stopResizing,
  };
}
