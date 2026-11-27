/**
 * INPUT: 当前会话首屏 durable 历史是否仍在加载（首次打开大会话时服务端会先同步回填）。
 * OUTPUT: 时间线为空期间与页面级水合占位同形的居中加载状态，避免长会话首开时呈现为空会话。
 * POS: DM/Room 面板共用的首屏历史占位；不发起请求。
 */
"use client";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceLoadingState } from "@/shared/ui/workspace/frame/workspace-loading-state";

export function ConversationHistoryLoadingState() {
  const { t } = useI18n();
  // 首开依次经过页面水合与会话历史两段等待；两段复用同一占位（尺寸、文案、居中方式），
  // 交接时只多出 Header/Composer，加载提示不再换位置、换文案。
  return <WorkspaceLoadingState label={t("room.loading_conversation")} />;
}
