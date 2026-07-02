"use client";

import { ExternalLink, Loader2, PackagePlus, Puzzle } from "lucide-react";

import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import { get_ui_button_class_name } from "@/shared/ui/button-styles";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { ExternalSkillSearchItem } from "@/types/capability/skill";

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

function formatInstalls(installs: number): string {
  if (installs >= 1000) {
    return `${(installs / 1000).toFixed(installs >= 100000 ? 0 : 1)}K`;
  }
  return `${installs}`;
}

export function ExternalSkillPreviewDialog({
  item,
  is_open: isOpen,
  busy,
  preview_loading: previewLoading,
  name_conflict: nameConflict = false,
  already_imported: alreadyImported,
  on_close: onClose,
  on_import_only: onImportOnly,
}: ExternalSkillPreviewDialogProps) {
  if (!isOpen || !item) return null;
  const isSkillsSh = item.source_kind === "skills_sh" || item.import_mode === "skills_sh";
  const previewMarkdown = previewLoading && !item.readme_markdown
    ? "正在加载预览内容..."
    : isSkillsSh
      ? "skills.sh 暂不提供内置预览，请打开原始页面查看。"
      : (item.readme_markdown || item.description || "暂无预览内容");
  const sourceLabel = item.source_name || item.source_kind || "社区";
  const sourceRef = item.package_spec || item.git_url || item.raw_url || item.source;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop class_name="z-[9999]" on_close={onClose}>
        <UiDialogShell class_name="h-[84vh]" size="xl">
          <UiDialogHeader
            icon={<Puzzle className="h-4 w-4" />}
            on_close={onClose}
            subtitle={`${sourceRef} · ${formatInstalls(item.installs)} 次安装`}
            title={item.title || item.skill_slug}
          />
          <UiDialogBody scrollable>
            <div className="mb-5 flex flex-wrap gap-2">
              <UiBadge size="xs">{sourceLabel}</UiBadge>
              {alreadyImported ? (
                <UiBadge size="xs" tone="success">已导入</UiBadge>
              ) : nameConflict ? (
                <UiBadge size="xs" tone="warning">同名冲突</UiBadge>
              ) : null}
            </div>
            <SkillMarkdown
              description={item.description}
              markdown={previewMarkdown}
              title={item.title || item.skill_slug}
            />
          </UiDialogBody>

          <UiDialogFooter class_name="flex-wrap justify-between gap-3">
            {item.detail_url ? (
              <a
                className={get_ui_button_class_name({ size: "sm", variant: "text" }, "w-fit")}
                href={item.detail_url}
                rel="noreferrer"
                target="_blank"
              >
                <ExternalLink className="h-4 w-4" />
                打开原始页面
              </a>
            ) : <span />}
            <div className="flex flex-wrap items-center gap-2">
              <UiButton
                disabled={busy || alreadyImported || nameConflict}
                onClick={onImportOnly}
                size="sm"
                tone="primary"
                type="button"
                variant="solid"
              >
                {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <PackagePlus className="h-4 w-4" />}
                导入到技能库
              </UiButton>
            </div>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
