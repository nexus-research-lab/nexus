/**
 * INPUT: Room 完整会话目录、当前会话与选择命令。
 * OUTPUT: 排除内部草稿、明确空历史、跟随语言显示时间并复用模态焦点/关闭协议的移动切换器。
 * POS: Room 窄窗历史投影视图；领域只拥有顶栏下拉几何与选择命令，模态行为归共享 Dialog。
 */

import { X } from "lucide-react";
import { useId, useRef } from "react";

import { formatRelativeTime } from "@/lib/format/relative-time";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { useDialogModalBehavior } from "@/shared/ui/dialog/dialog-behavior";
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
  const { locale, t } = useI18n();
  const titleId = useId();
  const rootRef = useRef<HTMLElement | null>(null);
  useDialogModalBehavior({ enabled: isOpen, onClose, rootRef });
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
        ref={rootRef}
        aria-labelledby={titleId}
        aria-modal="true"
        className={cn(
          "absolute inset-x-0 flex max-h-[56dvh] flex-col overflow-hidden rounded-b-2xl border-b border-[color:color-mix(in_srgb,var(--divider-subtle-color)_82%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_84%,var(--surface-panel-background)_16%)] shadow-(--surface-popover-shadow) backdrop-blur-[20px] animate-in fade-in-0 slide-in-from-top-2 duration-(--motion-duration-fast)",
          MOBILE_SHELL_HEADER_OFFSET_CLASS_NAME,
          getUiOverlayLayerClassName("dialog"),
        )}
        role="dialog"
        tabIndex={-1}
      >
        <header className="flex min-h-12 shrink-0 items-center justify-between gap-3 border-b divider-subtle px-4 py-2">
          <h2
            className={cn(
              "truncate",
              getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "semibold" }),
            )}
            id={titleId}
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
          {historyConversations.length === 0 ? (
            <p className={cn("px-4 py-6 text-center", getUiTypographyClassName({ role: "supporting", tone: "muted" }))} role="status">
              {t("room.no_conversations")}
            </p>
          ) : null}
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
                      role: "metadata",
                      tone: "muted",
                    }),
                  )}>
                    {formatRelativeTime(conversation.last_activity_at, locale)}
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
