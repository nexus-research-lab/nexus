/**
 * INPUT: Room 完整会话目录、当前会话与选择命令。
 * OUTPUT: 排除内部草稿但保留外部 Session 的移动端历史切换器。
 * POS: Room 窄窗历史投影视图，不维护标签目录、路由或选中真相。
 */

import { X } from "lucide-react";

import { formatRelativeTime } from "@/lib/format/relative-time";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiListRow } from "@/shared/ui/list/list-row";
import { MOBILE_SHELL_HEADER_OFFSET_CLASS_NAME } from "@/shared/ui/layout/mobile-shell-header-layout";
import { getUiOverlayLayerClassName } from "@/shared/ui/overlay/layer-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { RoomConversationView } from "@/types/conversation/conversation";

import { filterRoomHistoryConversations } from "../history/room-history-model";

interface RoomMobileConversationSwitcherProps {
  activeConversationId: string | null;
  conversations: RoomConversationView[];
  isOpen: boolean;
  onClose: () => void;
  onSelect: (conversationId: string) => void;
}

export function RoomMobileConversationSwitcher({
  activeConversationId,
  conversations,
  isOpen,
  onClose,
  onSelect,
}: RoomMobileConversationSwitcherProps) {
  const { t } = useI18n();
  const historyConversations = filterRoomHistoryConversations(conversations);
  if (!isOpen) {
    return null;
  }

  return (
    <>
      <button
        aria-label={t("common.close")}
        className={cn(
          "absolute inset-x-0 bottom-0 bg-(--dialog-backdrop-color) backdrop-blur-[1px] animate-in fade-in-0 duration-(--motion-duration-fast)",
          MOBILE_SHELL_HEADER_OFFSET_CLASS_NAME,
          getUiOverlayLayerClassName("dialogUnderlay"),
        )}
        onClick={onClose}
        type="button"
      />

      <section
        aria-labelledby="mobile-conversation-switcher-title"
        aria-modal="true"
        className={cn(
          "absolute inset-x-0 flex max-h-[56dvh] flex-col overflow-hidden rounded-b-2xl border-b border-[color:color-mix(in_srgb,var(--divider-subtle-color)_82%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_84%,var(--surface-panel-background)_16%)] shadow-(--surface-popover-shadow) backdrop-blur-[20px] animate-in fade-in-0 slide-in-from-top-2 duration-(--motion-duration-fast)",
          MOBILE_SHELL_HEADER_OFFSET_CLASS_NAME,
          getUiOverlayLayerClassName("dialog"),
        )}
        role="dialog"
      >
        <header className="flex min-h-12 shrink-0 items-center justify-between gap-3 border-b divider-subtle px-4 py-2">
          <h2
            className={cn(
              "truncate",
              getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "semibold" }),
            )}
            id="mobile-conversation-switcher-title"
          >
            {t("room.switch_conversation")}
          </h2>

          <UiIconButton
            aria-label={t("common.close")}
            onClick={onClose}
            size="md"
            variant="ghost"
          >
            <X className="h-4 w-4" />
          </UiIconButton>
        </header>

        <div className="soft-scrollbar min-h-0 flex-1 space-y-1 overflow-y-auto p-2.5">
          {historyConversations.map((conversation) => {
            const isActive = conversation.conversation_id === activeConversationId;
            return (
              <UiListRow
                key={conversation.conversation_id}
                active={isActive}
                activeTone="sidebar"
                aria-current={isActive ? "page" : undefined}
                className="overflow-hidden"
                density="compact"
                onClick={() => {
                  onSelect(conversation.conversation_id);
                  onClose();
                }}
              >
                <div className="min-w-0 flex-1">
                  <p className={cn(
                    "truncate",
                    getUiTypographyClassName({
                      role: "body",
                      tone: isActive ? "strong" : "default",
                      weight: isActive ? "semibold" : "medium",
                    }),
                  )}>
                    {conversation.title?.trim() || t("room.new_conversation")}
                  </p>
                  <span className={cn(
                    "mt-0.5 block",
                    getUiTypographyClassName({
                      role: "caption",
                      tone: isActive ? "muted" : "soft",
                    }),
                  )}>
                    {formatRelativeTime(conversation.last_activity_at)}
                  </span>
                </div>
              </UiListRow>
            );
          })}
        </div>
      </section>
    </>
  );
}
