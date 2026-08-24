/**
 * INPUT: 外部 Skill 预览模型与导入、关闭动作。
 * OUTPUT: 以正文为主、来源元数据为辅的 plain 预览弹窗。
 * POS: 社区 Skill 搜索结果的只读预览与导入入口。
 */
"use client";

import { ExternalLink, Loader2, PackagePlus } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { ExternalSkillSearchItem } from "@/types/capability/skill";

import { SkillMarkdown } from "../detail/skill-markdown";
import type {
  ExternalSkillImportModel,
  ExternalSkillPreviewModel,
} from "./external-skill-model";

interface ExternalSkillPreviewDialogProps {
  model: ExternalSkillPreviewModel | null;
  onClose: () => void;
  onImport: (item: ExternalSkillSearchItem) => void;
}

export function ExternalSkillPreviewDialog({
  model,
  onClose,
  onImport,
}: ExternalSkillPreviewDialogProps) {
  const { t } = useI18n();
  if (!model) return null;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" onClose={onClose}>
        <UiDialogShell className="h-[84vh]" size="xl">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={model.title}
          />
          <UiDialogBody scrollable>
            <div className="mb-5 flex flex-wrap gap-2">
              <UiBadge size="xs">{model.sourceLabel}</UiBadge>
              <ExternalSkillImportBadge importState={model.importState} />
            </div>
            <SkillMarkdown
              description={model.item.description}
              markdown={model.markdown}
              title={model.title}
            />
          </UiDialogBody>

          <UiDialogFooter appearance="plain" className="flex-wrap justify-between gap-3">
            {model.detailUrl ? (
              <a
                className={getUiButtonClassName({ size: "sm", variant: "text" }, "w-fit")}
                href={model.detailUrl}
                rel="noreferrer"
                target="_blank"
              >
                <ExternalLink className="h-4 w-4" />
                {t("capability.skills_external_open_original")}
              </a>
            ) : <span />}
            <div className="flex flex-wrap items-center gap-2">
              <UiButton
                disabled={model.importState.busy || !model.importState.canImport}
                onClick={() => onImport(model.item)}
                size="sm"
                tone="primary"
                type="button"
                variant="solid"
              >
                {model.importState.busy
                  ? <Loader2 className="h-4 w-4 animate-spin" />
                  : <PackagePlus className="h-4 w-4" />}
                {t("capability.skills_external_import_action")}
              </UiButton>
            </div>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

interface ExternalSkillImportBadgeProps {
  importState: ExternalSkillImportModel;
}

const IMPORT_BADGE_TONES = {
  conflict: "warning",
  imported: "success",
} as const;

function ExternalSkillImportBadge({
  importState,
}: ExternalSkillImportBadgeProps) {
  if (importState.kind === "available") return null;
  return (
    <UiBadge size="xs" tone={IMPORT_BADGE_TONES[importState.kind]}>
      {importState.label}
    </UiBadge>
  );
}
