"use client";

/**
 * INPUT: 当前 Room 类型与创建会话命令。
 * OUTPUT: DM/Group 共用的新会话空态入口。
 * POS: Room Surface 在用户明确关闭全部标签或尚无会话时展示的共享业务视图。
 */

import { FolderKanban, MessageSquarePlus } from "lucide-react";

import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceCatalogTextAction } from "@/shared/ui/workspace/catalog/workspace-catalog-actions";
import { WorkspaceCatalogCard } from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import {
  WorkspaceCatalogBody,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/catalog/workspace-catalog-content";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/catalog/workspace-icon-frame";

interface RoomConversationEmptyStateProps {
  isDm: boolean;
  onCreateConversation: (title?: string) => void | Promise<string | null>;
}

export function RoomConversationEmptyState({
  isDm,
  onCreateConversation,
}: RoomConversationEmptyStateProps) {
  const { t } = useI18n();
  const highlights = [
    t("room.empty_conversation_highlight_context"),
    t("room.empty_conversation_highlight_history"),
    t("room.empty_conversation_highlight_workspace"),
  ];

  return (
    <div className="flex min-h-0 flex-1 items-center justify-center p-6 sm:p-8">
      <WorkspaceCatalogCard className="w-full max-w-[56rem]" size="panel">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-[34rem]">
            <WorkspaceCatalogHeader className="items-center">
              <WorkspaceIconFrame className="h-12 w-12" shape="round" size="md" tone="primary">
                <FolderKanban className="h-6 w-6" />
              </WorkspaceIconFrame>
              <div>
                <p className="text-xs font-medium text-(--text-muted)">
                  {t(isDm ? "room.empty_conversation_tag_dm" : "room.empty_conversation_tag_group")}
                </p>
                <WorkspaceCatalogTitle as="h2" className="mt-1.5" size="lg">
                  {t("room.empty_conversation_title")}
                </WorkspaceCatalogTitle>
              </div>
            </WorkspaceCatalogHeader>

            <WorkspaceCatalogBody className="mt-4">
              <WorkspaceCatalogDescription className="max-w-[32rem]" lines={3} size="md">
                {t("room.empty_conversation_description")}
              </WorkspaceCatalogDescription>
            </WorkspaceCatalogBody>

            <WorkspaceCatalogFooter className="mt-4 flex-wrap gap-2.5" justify="start">
              <WorkspaceCatalogTextAction
                data-tour-anchor={CONVERSATION_TOUR_ANCHORS.empty_create}
                tone="primary"
                onClick={() => {
                  void onCreateConversation();
                }}
              >
                <MessageSquarePlus className="h-5 w-5" />
                {t("room.empty_conversation_create_action")}
              </WorkspaceCatalogTextAction>
            </WorkspaceCatalogFooter>
          </div>

          <div className="min-w-0 flex-1 lg:max-w-[22rem]">
            <p className="mb-2 text-xs font-medium text-(--text-muted)">
              {t("room.empty_conversation_highlight_label")}
            </p>
            <ul className="divide-y divide-(--divider-subtle-color) border-y border-(--divider-subtle-color)">
              {highlights.map((highlight) => (
                <li className="flex items-center gap-2.5 py-2.5 text-sm text-(--text-default)" key={highlight}>
                  <span aria-hidden="true" className="h-1.5 w-1.5 shrink-0 rounded-full bg-(--primary) opacity-70" />
                  <span>{highlight}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </WorkspaceCatalogCard>
    </div>
  );
}
