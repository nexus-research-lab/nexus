"use client";

import { useEffect, useState } from "react";

/**
 * 监听系统的动态效果偏好，避免高成本动画在低动态模式下继续运行。
 */
export function usePrefersReducedMotion() {
  const [prefers_reduced_motion, setPrefersReducedMotion] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }

    const media_query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update_preference = () => {
      setPrefersReducedMotion(media_query.matches);
    };

    update_preference();
    media_query.addEventListener("change", update_preference);

    return () => {
      media_query.removeEventListener("change", update_preference);
    };
  }, []);

  return prefers_reduced_motion;
}
