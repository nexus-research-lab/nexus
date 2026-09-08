// INPUT: Room 会话集合、路由选择、历史入口和页面生命周期命令。
// OUTPUT: 绑定当前 owner 标签/固定偏好的受控 WorkspaceConversationTabs。
// POS: DM、Room 与 Contacts 联络共用的业务适配；不复制标签 DOM、样式或滚动。

import { useMemo, type ReactNode } from "react";

import { getExternalSessionConversationLabel } from "@/lib/conversation/external-session";
import { useI18n } from "@/shared/i18n/i18n-context";
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

  return (
    <WorkspaceConversationTabs
      activeConversationId={controller.activeConversationId}
      isCreating={controller.isCreating}
      leadingControl={leadingControl}
      onCloseConversation={controller.closeConversation}
      onCreateConversation={options.onCreateConversation ? () => { void controller.createConversation(); } : undefined}
      onSelectConversation={controller.selectConversation}
      onTogglePin={(id) => {
        const conversation = controller.orderedConversations.find((item) => item.conversation_id === id);
        if (!conversation) return;
        togglePinnedConversation({
          conversation_id: id,
          room_id: conversation.room_id,
          session_key: conversation.session_key,
          title: conversation.title?.trim() || t("room.new_conversation"),
        });
      }}
      tabs={tabs}
      tourAnchor={tourAnchor}
    />
  );
}
