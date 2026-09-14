// INPUT: 调用方的鼠标移动投影与交互启用状态。
// OUTPUT: 本地拖动状态、开始/停止入口；松开、失焦、隐藏或停用后结束。
// POS: 中立鼠标拖动生命周期；尺寸、边界、持久化与业务命令归调用方。

import { useCallback, useEffect } from "react";

import { useResettableState } from "./use-resettable-state";

export function useMouseDrag(onMove: (event: MouseEvent) => void, enabled = true) {
  const [isDragging, setIsDragging] = useResettableState(false, enabled);
  const startDragging = useCallback(() => {
    if (enabled) setIsDragging(true);
  }, [enabled, setIsDragging]);
  const stopDragging = useCallback(() => setIsDragging(false), [setIsDragging]);

  useEffect(() => {
    if (!isDragging || !enabled) return;

    let stopped = false;
    const finish = () => {
      stopped = true;
      stopDragging();
    };
    const handleMove = (event: MouseEvent) => {
      if (stopped) return;
      // 松开可能发生在窗口外；下一次移动不能继续改变布局。
      if ((event.buttons & 1) === 0) finish();
      else onMove(event);
    };
    const handleUp = (event: MouseEvent) => {
      if ((event.buttons & 1) === 0) finish();
    };
    const handleVisibility = () => {
      if (document.visibilityState === "hidden") finish();
    };

    window.addEventListener("mousemove", handleMove);
    window.addEventListener("mouseup", handleUp);
    window.addEventListener("blur", finish);
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      stopped = true;
      window.removeEventListener("mousemove", handleMove);
      window.removeEventListener("mouseup", handleUp);
      window.removeEventListener("blur", finish);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, [enabled, isDragging, onMove, stopDragging]);

  return { isDragging: enabled && isDragging, startDragging, stopDragging };
}
