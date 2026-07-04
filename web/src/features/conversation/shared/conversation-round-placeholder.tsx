import { MessageSquareText } from "lucide-react";

import { formatRelativeTime } from "@/lib/utils";
import type { SessionRoundIndexItem } from "@/types/conversation/room";

function formatPlaceholderStatus(item?: SessionRoundIndexItem): string {
  if (item?.isLive) {
    return "处理中";
  }
  switch (item?.status) {
    case "error":
      return "失败";
    case "interrupted":
      return "已中断";
    case "success":
      return "已处理";
    default:
      return "未加载";
  }
}

function formatPlaceholderTime(item?: SessionRoundIndexItem): string {
  if (!item?.timestamp) {
    return "";
  }
  return formatRelativeTime(item.timestamp);
}

export function ConversationRoundPlaceholder({
  indexItem,
  roundId,
}: {
  indexItem?: SessionRoundIndexItem;
  roundId: string;
}) {
  const title = indexItem?.title?.trim() || `第 ${roundId.slice(0, 8)} 轮`;
  const time = formatPlaceholderTime(indexItem);
  const status = formatPlaceholderStatus(indexItem);

  return (
    <div className="mx-auto flex min-h-20 w-full max-w-[980px] items-center py-2">
      <div className="flex min-w-0 items-center gap-2 rounded-[8px] border border-(--surface-canvas-border) bg-(--surface-elevated-background)/72 px-3 py-2 text-[12px] text-(--text-muted) shadow-[0_10px_28px_rgba(15,23,42,0.04)]">
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[8px] bg-primary/8 text-primary">
          <MessageSquareText className="h-3.5 w-3.5" />
        </span>
        <span className="min-w-0">
          <span className="block max-w-[420px] truncate font-medium text-(--text-default)">
            {title}
          </span>
          <span className="mt-0.5 flex items-center gap-1.5 text-[11px] text-(--text-soft)">
            {time ? <span>{time}</span> : null}
            {time ? <span aria-hidden>·</span> : null}
            <span>{status}</span>
          </span>
        </span>
      </div>
    </div>
  );
}
