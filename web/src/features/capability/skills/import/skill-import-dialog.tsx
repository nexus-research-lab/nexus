"use client";

import { type ComponentType, type RefObject } from "react";
import { GitBranch, PackageCheck } from "lucide-react";

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

const MODE_ICONS: Record<
  SkillImportDialogMode,
  ComponentType<{ className?: string }>
> = {
  git: GitBranch,
  local: PackageCheck,
};

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
  const HeaderIcon = MODE_ICONS[mode];

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        onClose={controller.dismissAction}
      >
        <UiDialogFormShell
          className="max-h-[86vh]"
          onSubmit={controller.submit}
          size="xl"
        >
          <UiDialogHeader
            icon={<HeaderIcon className="h-4 w-4" />}
            onClose={controller.dismissAction}
            subtitle={t("capability.skills_import_subtitle")}
            title={t("capability.skills_import_title")}
          />
          <UiDialogBody
            className="grid min-h-0 gap-5 lg:grid-cols-[minmax(0,1fr)_340px]"
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
