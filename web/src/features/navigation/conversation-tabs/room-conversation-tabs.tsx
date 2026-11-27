// INPUT: Room 会话集合、路由选择、历史入口和页面生命周期命令。
// OUTPUT: 绑定当前 owner 标签/固定偏好的受控 WorkspaceConversationTabs；单个未固定会话不渲染标签，只保留固定图标。
// POS: DM、Room 与 Contacts 联络共用的业务适配；不复制标签 DOM、样式或滚动。

import { Pin } from "lucide-react";
import { useMemo, type ReactNode } from "react";

import { getExternalSessionConversationLabel } from "@/lib/conversation/external-session";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import {
  WorkspaceConversationTabs,
  type WorkspaceConversationTabItem,
} from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { useRoomNavigationStore } from "@/store/room-navigation";
import type { RoomConversationView } from "@/types/conversation/conversation";

import type { FinalConversationReplacementHandler } from "./final-conversation-replacement";
import { useRoomConversationTabs } from "./use-room-conversation-tabs";

interface RoomConversationTabsProps {
  conversations: RoomConversationView[];
  conversationId: string | null;
  leadingControl?: ReactNode;
  tourAnchor?: string;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation?: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
  onReplaceFinalConversation?: FinalConversationReplacementHandler;
  pinningEnabled?: boolean;
}

export function RoomConversationTabs({
  leadingControl,
  pinningEnabled = true,
  tourAnchor,
  ...options
}: RoomConversationTabsProps) {
  const { t } = useI18n();
  const pinnedConversations = useRoomNavigationStore((state) => state.pinned_conversations);
  const togglePinnedConversation = useRoomNavigationStore((state) => state.toggle_pinned_conversation);
  const controller = useRoomConversationTabs(options);
  const canClose = controller.orderedConversations.length > 1 || Boolean(options.onReplaceFinalConversation);
  const tabs = useMemo<WorkspaceConversationTabItem[]>(() => controller.orderedConversations.map((conversation) => ({
    id: conversation.conversation_id,
    title: conversation.title?.trim() || t("room.new_conversation"),
    canClose,
    canPin: pinningEnabled && Boolean(conversation.room_id.trim() && conversation.conversation_id.trim()),
    isPinned: pinnedConversations.some((item) => (
      item.room_id === conversation.room_id && item.conversation_id === conversation.conversation_id
    )),
    externalSessionLabel: getExternalSessionConversationLabel(conversation),
  })), [canClose, controller.orderedConversations, pinnedConversations, pinningEnabled, t]);
  // 只有一个未固定会话时，标签只是重复 Header 已显示的对象名；隐藏标签，保留历史与新建入口，
  // 固定动作收成标签位置上的单个图标。固定后恢复显示标签，取消固定沿用标签自身入口。
  const singleUnpinnedTab = tabs.length === 1 && !tabs[0].isPinned ? tabs[0] : null;
  const visibleTabs = singleUnpinnedTab ? [] : tabs;
  const togglePin = (id: string) => {
    const conversation = controller.orderedConversations.find((item) => item.conversation_id === id);
    if (!conversation) return;
    togglePinnedConversation({
      conversation_id: id,
      room_id: conversation.room_id,
      session_key: conversation.session_key,
      title: conversation.title?.trim() || t("room.new_conversation"),
    });
  };
  const leading = singleUnpinnedTab?.canPin ? (
    <>
      {leadingControl}
      <UiIconButton
        aria-label={t("room.pin_conversation")}
        aria-pressed={false}
        className="shrink-0"
        onClick={() => togglePin(singleUnpinnedTab.id)}
        size="md"
        tooltip={t("room.pin_conversation")}
        variant="ghost"
      >
        <Pin aria-hidden className="h-4 w-4" />
      </UiIconButton>
    </>
  ) : leadingControl;

  return (
    <WorkspaceConversationTabs
      activeConversationId={controller.activeConversationId}
      isCreating={controller.isCreating}
      leadingControl={leading}
      onCloseConversation={controller.closeConversation}
      onCreateConversation={options.onCreateConversation ? () => { void controller.createConversation(); } : undefined}
      onSelectConversation={controller.selectConversation}
      onTogglePin={togglePin}
      tabs={visibleTabs}
      tourAnchor={tourAnchor}
    />
  );
}
