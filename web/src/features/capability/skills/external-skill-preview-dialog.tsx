"use client";

import { ExternalLink, Loader2, PackagePlus, X } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  DIALOG_ICON_BUTTON_CLASS_NAME,
  DIALOG_TAG_CLASS_NAME,
  getDialogActionClassName,
  getDialogNoteClassName,
  getDialogNoteStyle,
} from "@/shared/ui/dialog/dialog-styles";
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
              {item.package_spec} · {formatInstalls(item.installs)} installs
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              <span className={DIALOG_TAG_CLASS_NAME}>
                社区技能
              </span>
              <span className={DIALOG_TAG_CLASS_NAME}>
                {item.source}
              </span>
              {already_imported ? (
                <span className={cn(DIALOG_TAG_CLASS_NAME, "text-emerald-700")}>
                  已导入
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
          <div className={getDialogNoteClassName("default", "mb-5")} style={getDialogNoteStyle("default")}>
            这是来自社区的外部技能预览。导入后会进入 Nexus 的技能目录，再在 Agent 设置里为具体智能体安装。
          </div>
          <SkillMarkdown markdown={item.readme_markdown || item.description || "暂无预览内容"} />
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
              className={getDialogActionClassName("primary")}
              disabled={busy || already_imported}
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
