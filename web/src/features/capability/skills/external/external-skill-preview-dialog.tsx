"use client";

import { ExternalLink, Loader2, PackagePlus, Puzzle } from "lucide-react";

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
  if (!model) return null;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" onClose={onClose}>
        <UiDialogShell className="h-[84vh]" size="xl">
          <UiDialogHeader
            icon={<Puzzle className="h-4 w-4" />}
            onClose={onClose}
            subtitle={model.subtitle}
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

          <UiDialogFooter className="flex-wrap justify-between gap-3">
            {model.detailUrl ? (
              <a
                className={getUiButtonClassName({ size: "sm", variant: "text" }, "w-fit")}
                href={model.detailUrl}
                rel="noreferrer"
                target="_blank"
              >
                <ExternalLink className="h-4 w-4" />
                打开原始页面
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
                导入到技能库
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
