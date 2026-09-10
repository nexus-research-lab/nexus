/**
 * INPUT: 只读资源失败后的结果、已有内容影响和安全恢复动作。
 * OUTPUT: 不自动消失、不抢焦点的紧凑 Problem / Impact / Recovery 提示。
 * POS: Conversation 读取资源共用的可靠性视图；不解析错误文本，也不触发自动重试。
 */
"use client";

import { CircleAlert, RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiInlineNotice,
  type UiInlineNoticeVariant,
} from "@/shared/ui/feedback/inline-notice";

export function ReadResourceReliabilityNotice({
  className,
  impact,
  isRefreshing,
  onRefresh,
  problem,
  resource,
  stale,
  variant = "edge",
}: {
  className?: string;
  impact: string;
  isRefreshing: boolean;
  onRefresh: () => void;
  problem: string;
  resource: string;
  stale: boolean;
  variant?: UiInlineNoticeVariant;
}) {
  const { t } = useI18n();
  return (
    <UiInlineNotice
      action={{
        icon: <RefreshCw />,
        label: t("state.reload_check"),
        onClick: onRefresh,
        pending: isRefreshing,
      }}
      aria-label={problem}
      className={className}
      data-read-resource={resource}
      data-read-resource-state={stale ? "stale" : "error"}
      icon={<CircleAlert />}
      message={impact}
      title={problem}
      tone="warning"
      variant={variant}
    />
  );
}
