// INPUT: 浏览器系统动效偏好和组件订阅生命周期。
// OUTPUT: 从首次渲染起生效并持续更新的减少动效偏好；不支持媒体查询时保留默认关闭。
// POS: 共享浏览器偏好 Hook，不决定组件的动画样式或业务状态。
"use client";

import { useMediaQuery } from "./use-media-query";

/**
 * 监听系统的动态效果偏好，避免高成本动画在低动态模式下继续运行。
 */
export function usePrefersReducedMotion() {
  return useMediaQuery("(prefers-reduced-motion: reduce)");
}
