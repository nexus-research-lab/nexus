import { cn } from "@/shared/ui/class-name";

import createGroupChatIconSource from "./create-group-chat-icon.png";

/** 中文注释：创建群聊入口共用同一图形与主题对比度，消费者只决定展示尺寸。 */
export function CreateGroupChatIcon({ className }: { className?: string }) {
  return (
    <img
      alt=""
      aria-hidden="true"
      className={cn(
        "select-none brightness-[0.72] contrast-[1.24] saturate-[1.12] dark:brightness-[1.12] dark:contrast-100",
        className,
      )}
      draggable={false}
      src={createGroupChatIconSource}
    />
  );
}
