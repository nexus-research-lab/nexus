"use client";

import { ExternalLink, Loader2, PackagePlus, X } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_TAG_CLASS_NAME,
  get_dialog_action_class_name,
} from "@/shared/ui/dialog/dialog-styles";
import { ExternalSkillSearchItem } from "@/types/skill";

import { SkillMarkdown } from "./skill-markdown";

interface ExternalSkillPreviewDialogProps {
  item: ExternalSkillSearchItem | null;
  is_open: boolean;
  busy: boolean;
  preview_loading: boolean;
  name_conflict?: boolean;
  already_imported: boolean;
  on_close: () => void;
  on_import_only: () => void;
}

function format_installs(installs: number): string {
  if (installs >= 1000) {
    return `${(installs / 1000).toFixed(installs >= 100000 ? 0 : 1)}K`;
  }
  return `${installs}`;
}

export function ExternalSkillPreviewDialog({
  item,
  is_open,
  busy,
  preview_loading,
  name_conflict = false,
  already_imported,
  on_close,
  on_import_only,
}: ExternalSkillPreviewDialogProps) {
  if (!is_open || !item) return null;
  const preview_markdown = preview_loading && !item.readme_markdown
    ? "正在加载预览内容..."
    : (item.readme_markdown || item.description || "暂无预览内容");

  return (
    <div
      className="dialog-backdrop"
      onClick={on_close}
    >
      <div
        className="dialog-shell radius-shell-xl flex h-[84vh] w-full max-w-5xl flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h2 className="dialog-title truncate" data-size="hero">
              {item.title || item.skill_slug}
            </h2>
            <p className="dialog-subtitle">
              {item.package_spec} · {format_installs(item.installs)} installs
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              <span className={cn(DIALOG_TAG_CLASS_NAME, "gap-0 px-3 py-1 text-[10px] font-semibold tracking-[0.16em] uppercase")}>
                社区技能
              </span>
              {already_imported ? (
                <span className={cn(DIALOG_TAG_CLASS_NAME, "gap-0 px-3 py-1 text-[10px] font-semibold tracking-[0.16em] uppercase text-emerald-700")}>
                  已导入
                </span>
              ) : name_conflict ? (
                <span className={cn(DIALOG_TAG_CLASS_NAME, "gap-0 px-3 py-1 text-[10px] font-semibold tracking-[0.16em] uppercase text-amber-700")}>
                  同名冲突
                </span>
              ) : null}
            </div>
          </div>
          <button
            className={DIALOG_ICON_BUTTON_CLASS_NAME}
            aria-label="关闭"
            onClick={on_close}
            type="button"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="dialog-body dialog-body--scroll soft-scrollbar flex-1">
          <SkillMarkdown
            description={item.description}
            markdown={preview_markdown}
            title={item.title || item.skill_slug}
          />
        </div>

        <div className="dialog-footer flex-wrap justify-between gap-3">
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
            <button
              className={get_dialog_action_class_name("primary")}
              disabled={busy || already_imported || name_conflict}
              onClick={on_import_only}
              type="button"
            >
              {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <PackagePlus className="h-4 w-4" />}
              导入到技能库
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
