"use client";

/**
 * INPUT: runtime 已接收的 Markdown 快照与流式状态。
 * OUTPUT: 不叠加前端打字机节奏的原始快照。
 * POS: 保留统一调用边界；流速由真实 transport 决定，空间连续性由会话滚动层负责。
 */
export function useSmoothStreamingMarkdownContent(
  content: string,
  _enabled: boolean,
): string {
  return content;
}
