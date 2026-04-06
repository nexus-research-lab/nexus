"use client";

import { FolderKanban, MessageSquarePlus, Sparkles } from "lucide-react";

import { getStageGlowStyle } from "@/shared/ui/layout/stage-glow";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";

const EMPTY_STATE_SHELL_CLASS_NAME =
  "surface-card radius-shell-xl relative w-full overflow-hidden p-10 text-center";
const EMPTY_STATE_ICON_CLASS_NAME =
  "chip-default radius-shell-md relative inline-flex h-24 w-24 items-center justify-center text-slate-900/76";
const METRIC_CARD_CLASS_NAME = "surface-card radius-shell-md px-4 py-4";

interface RoomConversationEmptyStateProps {
  on_create_conversation: (title?: string) => void | Promise<string | null>;
}

export function RoomConversationEmptyState({
  on_create_conversation,
}: RoomConversationEmptyStateProps) {
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className={EMPTY_STATE_SHELL_CLASS_NAME}>
        <div className="pointer-events-none absolute inset-0 glass-grid opacity-16" />
        <div
          className="pointer-events-none absolute left-12 top-12 h-28 w-28 rounded-full opacity-32 blur-[8px]"
          style={getStageGlowStyle("lilac")}
        />
        <div
          className="pointer-events-none absolute bottom-10 right-12 h-28 w-28 rounded-full opacity-28 blur-[14px]"
          style={getStageGlowStyle("green")}
        />
        <div className="flex justify-center">
          <div className={EMPTY_STATE_ICON_CLASS_NAME}>
            <div className="absolute -right-2 -top-2 rounded-full bg-[linear-gradient(135deg,rgba(255,194,148,0.92),rgba(255,155,86,0.88))] p-2 text-[#8a4409] shadow-[0_14px_24px_rgba(255,157,86,0.24)]">
              <FolderKanban className="h-4 w-4" />
            </div>
            <Sparkles className="h-10 w-10" />
          </div>
        </div>

        <div className="mt-8 space-y-3">
          <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-slate-700/46">
            Room Collaboration
          </p>
          <h2 className="text-4xl font-extrabold tracking-[-0.05em] text-slate-950/90">
            从这里开始一段协作
          </h2>
          <p className="mx-auto max-w-md text-sm leading-7 text-slate-700/62">
            创建新会话后，这个工作台会进入真正承载协作的 room 对话态：对话、文件、计划与上下文围绕同一条任务展开。
          </p>
        </div>

        <div className="mt-8 grid gap-3 text-sm text-slate-700/62 md:grid-cols-3">
          <div className={METRIC_CARD_CLASS_NAME}>
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary" />
            <span>按 Room 承接协作任务</span>
          </div>
          <div className={METRIC_CARD_CLASS_NAME}>
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary" />
            <span>对话线程自动保存</span>
          </div>
          <div className={METRIC_CARD_CLASS_NAME}>
            <div className="mx-auto mb-2 h-2 w-2 rounded-full bg-primary" />
            <span>上下文围绕同一任务展开</span>
          </div>
        </div>

        <div className="mt-8 flex justify-center">
          <WorkspacePillButton
            onClick={() => {
              void on_create_conversation();
            }}
            size="lg"
            variant="primary"
          >
            <MessageSquarePlus className="h-5 w-5" />
            <span>创建新会话</span>
          </WorkspacePillButton>
        </div>

        <p className="mt-5 text-xs text-slate-700/52">
          先进入目标 room，再创建第一条对话
        </p>
      </div>
    </div>
  );
}
