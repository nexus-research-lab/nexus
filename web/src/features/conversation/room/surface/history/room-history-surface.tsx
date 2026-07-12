"use client";

import { useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type { RoomConversationView } from "@/types/conversation/conversation";

import { RoomHistoryEmptyState } from "./room-history-empty-state";
import { RoomHistoryItem } from "./room-history-item";
import { buildRoomHistoryEntries } from "./room-history-model";

interface RoomHistorySurfaceProps {
  canManageConversations?: boolean;
  conversations: RoomConversationView[];
  conversationId: string | null;
  currentRoomType: string;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
}

export function RoomHistorySurface({
  canManageConversations = true,
  conversations,
  conversationId,
  currentRoomType,
  onCreateConversation,
  onDeleteConversation,
  onSelectConversation,
  onUpdateConversationTitle,
}: RoomHistorySurfaceProps) {
  const {t} = useI18n();
  const [pendingDeleteConversation, setPendingDeleteConversation] = useState<RoomConversationView | null>(null);
  const entries = useMemo(() => buildRoomHistoryEntries({
    conversations,
    currentConversationId: conversationId,
    canManageConversations,
    canUpdateConversationTitle: onUpdateConversationTitle !== undefined,
  }), [
    canManageConversations,
    conversationId,
    conversations,
    onUpdateConversationTitle,
  ]);

  return (
    <>
      <WorkspaceSurfaceView
        bodyClassName="px-4 py-3.5 sm:px-5 xl:px-6"
        contentClassName="space-y-1.5"
        maxWidthClassName="max-w-none"
        title={currentRoomType === "dm" ? t("room.history_view_title_dm") : t("room.history_view_title")}
      >
        {entries.length > 0 ? (
          <div className="space-y-1.5">
            {entries.map((entry) => (
              <RoomHistoryItem
                entry={entry}
                key={entry.conversation.conversation_id}
                onDelete={() => setPendingDeleteConversation(entry.conversation)}
                onRename={(title) => {
                  void onUpdateConversationTitle?.(entry.conversation.conversation_id, title);
                }}
                onSelect={() => onSelectConversation(entry.conversation.conversation_id)}
              />
            ))}
          </div>
        ) : (
          <RoomHistoryEmptyState
            canCreateConversation={canManageConversations}
            onCreateConversation={() => {
              void onCreateConversation();
            }}
          />
        )}
      </WorkspaceSurfaceView>

      <ConfirmDialog
        confirmText={t("common.delete")}
        isOpen={Boolean(pendingDeleteConversation)}
        message={t("room.delete_conversation_message", {
          title: pendingDeleteConversation?.title?.trim() || t("room.untitled_conversation"),
        })}
        onCancel={() => setPendingDeleteConversation(null)}
        onConfirm={() => {
          const target = pendingDeleteConversation;
          setPendingDeleteConversation(null);
          if (target) {
            void onDeleteConversation(target.conversation_id);
          }
        }}
        title={t("room.delete_conversation_title")}
        variant="danger"
      />
    </>
  );
}
