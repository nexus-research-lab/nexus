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

interface GroupConversationEmptyStateProps {
  on_create_conversation: (title?: string) => void | Promise<string | null>;
}

const HIGHLIGHTS = [
  "围绕同一 room 承接任务",
  "消息线程和上下文自动保存",
  "文件与协作视图共用同一工作区",
] as const;

export function GroupConversationEmptyState({
  on_create_conversation,
}: GroupConversationEmptyStateProps) {
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
                  Room Collaboration
                </WorkspaceCatalogTag>
                <WorkspaceCatalogTitle as="h2" class_name="mt-3" size="hero">
                  让这间协作空间进入第一段对话。
                </WorkspaceCatalogTitle>
              </div>
            </WorkspaceCatalogHeader>

            <WorkspaceCatalogBody class_name="mt-5">
              <WorkspaceCatalogDescription lines={3} size="md">
                Room 不再只是一个容器，它会承接会话历史、线程、文件和多人协作视图。创建第一条对话后，当前工作区就会进入真正的协作态。
              </WorkspaceCatalogDescription>
            </WorkspaceCatalogBody>

            <WorkspaceCatalogFooter class_name="mt-6 flex-wrap gap-2.5" justify="start">
              <WorkspaceCatalogTextAction
                tone="primary"
                onClick={() => {
                  void on_create_conversation();
                }}
              >
                <MessageSquarePlus className="h-5 w-5" />
                创建新会话
              </WorkspaceCatalogTextAction>
            </WorkspaceCatalogFooter>
          </div>

          <div className="grid min-w-0 flex-1 gap-3 lg:max-w-[22rem]">
            {HIGHLIGHTS.map((highlight) => (
              <WorkspaceCatalogCard key={highlight} size="stat">
                <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
                  Capability
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
