/**
 * INPUT: 当前会话首屏 durable 历史是否仍在加载（含后端 indexing 重试）。
 * OUTPUT: 时间线为空期间的居中加载状态，避免长会话首开时呈现为空会话。
 * POS: DM/Room 面板共用的首屏历史占位；不发起请求，也不解释索引协议。
 */
"use client";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";

export function ConversationHistoryLoadingState() {
  const { t } = useI18n();
  return (
    <UiResourceState
      className="py-16"
      description={t("room.loading_conversation_detail")}
      size="sm"
      state="loading"
      title={t("room.loading_conversation")}
      variant="plain"
    />
  );
}
