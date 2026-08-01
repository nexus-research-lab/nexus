import {
  ArrowLeft,
  LoaderCircle,
  Pencil,
  Save,
  X,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import type { MemoryDocument } from "@/types/memory/memory";

import { formatMemoryModifiedTime } from "../memory-utils";
import {
  buildMemoryDocumentHeaderModel,
  type MemoryDocumentHeaderAction,
} from "./memory-document-model";

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
  const model = buildMemoryDocumentHeaderModel({
    dirty: controller.dirty,
    editing: controller.editing,
    isSaving: controller.isSaving,
    runtimeWriting,
  });
  return (
    <div className="shrink-0">
      <div className="nexus-memory-document-content flex min-h-[60px] items-center gap-3 py-3">
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
            <h2
              className="truncate text-[14px] font-semibold text-(--text-strong)"
              title={document.path}
            >
              {document.title}
            </h2>
            {runtimeWriting ? <MemoryRuntimeWritingStatus /> : null}
          </div>
          <div className="mt-0.5 text-xs text-(--text-soft)">
            {formatMemoryModifiedTime(document.modified_at, locale)}
          </div>
        </div>
        <MemoryHeaderActions action={model.action} controller={controller} />
      </div>
    </div>
  );
}

function MemoryRuntimeWritingStatus() {
  const { t } = useI18n();
  return (
    <span className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-(--primary)">
      <LoaderCircle className="h-3 w-3 animate-spin" />
      {t("capability.memory_runtime_writing")}
    </span>
  );
}

function MemoryHeaderActions({
  action,
  controller,
}: {
  action: MemoryDocumentHeaderAction;
  controller: MemoryDocumentHeaderController;
}) {
  const { t } = useI18n();
  if (action.kind === "edit") {
    return (
      <UiIconButton
        aria-label={t("common.edit")}
        className="shrink-0"
        disabled={action.disabled}
        onClick={controller.startEditing}
        size="md"
        title={t("common.edit")}
        variant="ghost"
      >
        <Pencil className="h-4 w-4" />
      </UiIconButton>
    );
  }
  const SaveIcon = action.saving ? LoaderCircle : Save;
  return (
    <div className="flex shrink-0 items-center gap-1.5">
      <UiButton
        disabled={action.saveDisabled}
        onClick={() => void controller.save()}
        size="sm"
      >
        <SaveIcon className={action.saving ? "h-3.5 w-3.5 animate-spin" : "h-3.5 w-3.5"} />
        {t("common.save")}
      </UiButton>
      <UiIconButton
        aria-label={t("common.cancel")}
        disabled={action.cancelDisabled}
        onClick={controller.cancelEditing}
        size="md"
        title={t("common.cancel")}
        variant="ghost"
      >
        <X className="h-4 w-4" />
      </UiIconButton>
    </div>
  );
}
