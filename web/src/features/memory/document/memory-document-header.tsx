import {
  ArrowLeft,
  FileText,
  Link2,
  LoaderCircle,
  Pencil,
  Save,
  X,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import type { MemoryDocument } from "@/types/memory/memory";

import {
  formatMemoryFileSize,
  formatMemoryModifiedTime,
} from "../memory-utils";

interface MemoryDocumentHeaderController {
  cancelEditing: () => void;
  dirty: boolean;
  editing: boolean;
  isSaving: boolean;
  save: () => Promise<void>;
  startEditing: () => void;
}

interface MemoryDocumentHeaderProps {
  controller: MemoryDocumentHeaderController;
  document: MemoryDocument;
  locale: string;
  onBack: () => void;
  runtimeWriting: boolean;
}

export function MemoryDocumentHeader({
  controller,
  document,
  locale,
  onBack,
  runtimeWriting,
}: MemoryDocumentHeaderProps) {
  const { t } = useI18n();
  return (
    <div className="flex min-h-[62px] shrink-0 items-center gap-3 border-b border-(--divider-subtle-color) px-4 py-2.5">
      <UiIconButton
        aria-label={t("common.back")}
        className="nexus-memory-compact-only"
        onClick={onBack}
        size="md"
        title={t("common.back")}
        variant="ghost"
      >
        <ArrowLeft className="h-4 w-4" />
      </UiIconButton>
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <FileText className="h-4 w-4 shrink-0 text-(--icon-muted)" />
          <h2 className="truncate text-[14px] font-semibold text-(--text-strong)">
            {document.title}
          </h2>
          {document.indexed && document.kind === "topic" ? (
            <span className="inline-flex shrink-0 items-center gap-1 text-[10.5px] font-medium text-emerald-600 dark:text-emerald-400">
              <Link2 className="h-3 w-3" />
              {t("capability.memory_indexed")}
            </span>
          ) : null}
          {runtimeWriting ? (
            <span className="inline-flex shrink-0 items-center gap-1 text-[10.5px] font-medium text-(--primary)">
              <LoaderCircle className="h-3 w-3 animate-spin" />
              {t("capability.memory_runtime_writing")}
            </span>
          ) : null}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-2 text-[11px] text-(--text-soft)">
          <span className="truncate font-mono">{document.path}</span>
          <span aria-hidden="true">·</span>
          <span className="shrink-0">{formatMemoryFileSize(document.size)}</span>
          <span aria-hidden="true">·</span>
          <span className="shrink-0">
            {formatMemoryModifiedTime(document.modified_at, locale)}
          </span>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        {controller.editing ? (
          <>
            <UiButton
              disabled={!controller.dirty || controller.isSaving || runtimeWriting}
              onClick={() => void controller.save()}
              size="sm"
            >
              {controller.isSaving
                ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                : <Save className="h-3.5 w-3.5" />}
              {t("common.save")}
            </UiButton>
            <UiIconButton
              aria-label={t("common.cancel")}
              disabled={controller.isSaving}
              onClick={controller.cancelEditing}
              size="md"
              title={t("common.cancel")}
              variant="ghost"
            >
              <X className="h-4 w-4" />
            </UiIconButton>
          </>
        ) : (
          <UiIconButton
            aria-label={t("common.edit")}
            disabled={runtimeWriting}
            onClick={controller.startEditing}
            size="md"
            title={t("common.edit")}
            variant="ghost"
          >
            <Pencil className="h-4 w-4" />
          </UiIconButton>
        )}
      </div>
    </div>
  );
}
