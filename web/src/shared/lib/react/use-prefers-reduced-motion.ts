// INPUT: 浏览器系统动效偏好和组件订阅生命周期。
// OUTPUT: 当前减少动效偏好；不支持媒体查询的环境保留静态初值。
// POS: 共享浏览器偏好 Hook，不决定组件的动画样式或业务状态。
"use client";

import { useEffect, useState } from "react";

/**
 * 监听系统的动态效果偏好，避免高成本动画在低动态模式下继续运行。
 */
export function usePrefersReducedMotion() {
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }

    const mediaQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
    const updatePreference = () => {
      setPrefersReducedMotion(mediaQuery.matches);
    };

    updatePreference();
    mediaQuery.addEventListener("change", updatePreference);

    return () => {
      mediaQuery.removeEventListener("change", updatePreference);
    };
  }, []);

  return prefersReducedMotion;
}
