/**
 * INPUT: Skill 导入模式、Git 草稿、本地文件入口与提交状态。
 * OUTPUT: 单列导入表单和按需展开的格式要求。
 * POS: 技能市场导入边界，不复制导入协议说明到标题区。
 */
"use client";

import { type RefObject } from "react";

import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";
import { SkillImportFooter } from "./skill-import-footer";
import { SkillImportGuide } from "./skill-import-guide";
import { SkillImportSource } from "./skill-import-source";
import { useSkillImportDialog } from "./use-skill-import-dialog";

interface SkillImportDialogProps {
  fileInputRef: RefObject<HTMLInputElement | null>;
  importing: boolean;
  mode: SkillImportDialogMode | null;
  onClose: () => void;
  onImportGit: (url: string, branch?: string, path?: string) => void;
  onSelectMode: (mode: SkillImportDialogMode) => void;
}

export function SkillImportDialog({
  fileInputRef,
  importing,
  mode,
  onClose,
  onImportGit,
  onSelectMode,
}: SkillImportDialogProps) {
  const { t } = useI18n();
  const controller = useSkillImportDialog({
    importing,
    mode,
    onClose,
    onImportGit,
  });
  if (!mode) {
    return null;
  }
  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialog"
        onClose={controller.dismissAction}
      >
        <UiDialogFormShell
          onSubmit={controller.submit}
          size="md"
          viewport="adaptiveMax"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={controller.dismissAction}
            title={t("capability.skills_import_title")}
          />
          <UiDialogBody
            className="min-h-0 space-y-5"
            scrollable
          >
            <SkillImportSource
              draft={controller.draft}
              fileInputRef={fileInputRef}
              gitUrlInputRef={controller.gitUrlInputRef}
              importing={importing}
              mode={mode}
              onSelectMode={onSelectMode}
              setDraftField={controller.setDraftField}
            />
            <SkillImportGuide importing={importing} />
          </UiDialogBody>
          <SkillImportFooter
            importing={importing}
            mode={mode}
            onClose={controller.close}
          />
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
