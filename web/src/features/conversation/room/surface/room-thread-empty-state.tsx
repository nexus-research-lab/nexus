// INPUT: Thread 面板模型已经确认的加载/无额外详情状态与当前语言。
// OUTPUT: 复用公共状态块、播报和几何的克制行内说明；不新增读取或动作。
// POS: 桌面/窄窗 Thread 空态适配，内容与加载判定仍归 Thread model。

"use client";

import { Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function RoomThreadEmptyState({ isLoading }: { isLoading: boolean }) {
  const { t } = useI18n();
  return (
    <UiResourceState
      icon={false}
      size="sm"
      state={isLoading ? "loading" : "empty"}
      variant="plain"
      title={(
        <span className={cn("flex items-center justify-center gap-2", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
          {isLoading ? <Loader2 aria-hidden="true" className={getUiSpinnerClassName({ size: "md", tone: "muted" })} /> : null}
          <span className="min-w-0">{t(isLoading ? "room.thread_waiting" : "room.thread_empty")}</span>
        </span>
      )}
    />
  );
}
