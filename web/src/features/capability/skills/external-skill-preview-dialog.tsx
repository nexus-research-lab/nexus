"use client";

import { ExternalLink, Loader2, PackagePlus, X } from "lucide-react";

import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { ExternalSkillSearchItem } from "@/types/skill";

import { SkillMarkdown } from "./skill-markdown";

interface ExternalSkillPreviewDialogProps {
  item: ExternalSkillSearchItem | null;
  is_open: boolean;
  busy: boolean;
  already_imported: boolean;
  on_close: () => void;
  on_import_only: () => void;
}

function formatInstalls(installs: number): string {
  if (installs >= 1000) {
    return `${(installs / 1000).toFixed(installs >= 100000 ? 0 : 1)}K`;
  }
  return `${installs}`;
}

export function ExternalSkillPreviewDialog({
  item,
  is_open,
  busy,
  already_imported,
  on_close,
  on_import_only,
}: ExternalSkillPreviewDialogProps) {
  if (!is_open || !item) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
      onClick={on_close}
    >
      <div
        className="modal-dialog-surface radius-shell-xl flex h-[84vh] w-full max-w-5xl flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4 border-b modal-divider px-6 py-5">
          <div className="min-w-0">
            <h2 className="truncate text-[28px] font-black tracking-[-0.04em] text-slate-950/92">
              {item.title || item.skill_slug}
            </h2>
            <p className="mt-1 text-[14px] text-slate-700/70">
              {item.package_spec} · {formatInstalls(item.installs)} installs
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              <span className="inline-flex rounded-full bg-white/70 px-3 py-1 text-[11px] font-semibold text-slate-600">
                社区技能
              </span>
              <span className="inline-flex rounded-full bg-white/70 px-3 py-1 text-[11px] font-semibold text-slate-600">
                {item.source}
              </span>
              {already_imported ? (
                <span className="inline-flex rounded-full bg-emerald-50 px-3 py-1 text-[11px] font-semibold text-emerald-600">
                  已导入
                </span>
              ) : null}
            </div>
          </div>
          <button
            aria-label="关闭"
            className="rounded-full p-2 text-slate-400 transition-colors hover:bg-white/60 hover:text-slate-700"
            onClick={on_close}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="soft-scrollbar flex-1 overflow-y-auto px-6 py-5">
          <div className="workspace-card mb-5 rounded-[22px] px-5 py-4 text-[13px] leading-6 text-slate-700/78">
            这是来自社区的外部技能预览。导入后会进入 Nexus 的全局技能库，再在 Agent 设置里为具体智能体启用。
          </div>
          <SkillMarkdown markdown={item.readme_markdown || item.description || "暂无预览内容"} />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 border-t modal-divider px-6 py-4">
          <a
            className="inline-flex items-center gap-2 text-sm font-semibold text-sky-600 underline decoration-sky-300 underline-offset-4"
            href={item.detail_url}
            rel="noreferrer"
            target="_blank"
          >
            <ExternalLink className="h-4 w-4" />
            打开原始页面
          </a>
          <div className="flex flex-wrap items-center gap-2">
            <WorkspacePillButton disabled={busy || already_imported} onClick={on_import_only}>
              {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <PackagePlus className="h-4 w-4" />}
              导入到技能库
            </WorkspacePillButton>
          </div>
        </div>
      </div>
    </div>
  );
}
