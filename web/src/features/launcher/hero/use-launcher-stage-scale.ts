"use client";

import { useLayoutEffect, useRef, useState } from "react";

// 中文注释：Hero 是固定尺寸的设计画布，响应式只靠这一个缩放系数收敛，
// 不再按视口宽高逐档打补丁。宽度按内容列（标题/输入框）适配；高度只对
// 云朵预算——Token 堆锚定视口底部、共用系数，不挤压云朵的可用高度。
const CONTENT_WIDTH = 560;
const DESIGN_HEIGHT = 520;
const MIN_SCALE = 0.3;
const MAX_SCALE = 1.00;

interface LauncherStageScale {
  scale: number;
  viewportRef: React.RefObject<HTMLDivElement | null>;
}

export function useLauncherStageScale(): LauncherStageScale {
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const [scale, setScale] = useState(1);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || typeof ResizeObserver === "undefined") {
      return;
    }
    const sync = () => {
      const { clientHeight, clientWidth } = viewport;
      if (!clientWidth || !clientHeight) {
        return;
      }
      const fit = Math.min(
        clientWidth / CONTENT_WIDTH,
        clientHeight / DESIGN_HEIGHT,
      );
      setScale(Math.min(MAX_SCALE, Math.max(MIN_SCALE, fit)));
    };
    sync();
    const observer = new ResizeObserver(sync);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, []);

  return { scale, viewportRef };
}
