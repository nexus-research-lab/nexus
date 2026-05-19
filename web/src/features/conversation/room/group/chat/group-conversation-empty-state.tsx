/**
 * =====================================================
 * @File   : group-conversation-empty-state.tsx
 * @Date   : 2026-04-11 16:39
 * @Author : leemysw
 * 2026-04-11 16:39   Create
 * =====================================================
 */

"use client";

import { FolderKanban, MessageSquarePlus } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceCatalogBody,
  WorkspaceCatalogCard,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceCatalogTag,
  WorkspaceCatalogTextAction,
  WorkspaceCatalogTitle,
  WorkspaceIconFrame,
} from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import { CONVERSATION_TOUR_ANCHORS } from "../../room-tour";

interface GroupConversationEmptyStateProps {
  on_create_conversation: (title?: string) => void | Promise<string | null>;
}

export function GroupConversationEmptyState({
  on_create_conversation,
}: GroupConversationEmptyStateProps) {
  const { t } = useI18n();
  const highlights = [
    t("room.empty_group_highlight_members"),
    t("room.empty_group_highlight_context"),
    t("room.empty_group_highlight_workspace"),
  ];

  return (
    <div className="flex min-h-0 flex-1 items-center justify-center p-6 sm:p-8">
      <WorkspaceCatalogCard class_name="w-full max-w-[56rem]" size="hero">
        <div className="flex flex-col gap-8 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-[34rem]">
            <WorkspaceCatalogHeader class_name="items-center">
              <WorkspaceIconFrame class_name="h-16 w-16" shape="round" size="lg" tone="primary">
                <FolderKanban className="h-7 w-7" />
              </WorkspaceIconFrame>
              <div>
                <WorkspaceCatalogTag class_name="text-[11px] font-semibold uppercase tracking-[0.14em]">
                  {t("room.empty_group_tag")}
                </WorkspaceCatalogTag>
                <WorkspaceCatalogTitle as="h2" class_name="mt-3" size="hero">
                  {t("room.empty_group_title")}
                </WorkspaceCatalogTitle>
              </div>
            </WorkspaceCatalogHeader>

            <WorkspaceCatalogBody class_name="mt-5">
              <WorkspaceCatalogDescription lines={3} size="md">
                {t("room.empty_group_description")}
              </WorkspaceCatalogDescription>
            </WorkspaceCatalogBody>

            <WorkspaceCatalogFooter class_name="mt-6 flex-wrap gap-2.5" justify="start">
              <WorkspaceCatalogTextAction
                data-tour-anchor={CONVERSATION_TOUR_ANCHORS.empty_create}
                tone="primary"
                onClick={() => {
                  void on_create_conversation();
                }}
              >
                <MessageSquarePlus className="h-5 w-5" />
                {t("room.empty_group_create_action")}
              </WorkspaceCatalogTextAction>
            </WorkspaceCatalogFooter>
          </div>

          <div className="grid min-w-0 flex-1 gap-3 lg:max-w-[22rem]">
            {highlights.map((highlight) => (
              <WorkspaceCatalogCard key={highlight} size="stat">
                <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
                  {t("room.empty_group_highlight_label")}
                </p>
                <WorkspaceCatalogTitle class_name="mt-2" size="sm">
                  {highlight}
                </WorkspaceCatalogTitle>
              </WorkspaceCatalogCard>
            ))}
          </div>
        </div>
      </WorkspaceCatalogCard>
    </div>
  );
}
