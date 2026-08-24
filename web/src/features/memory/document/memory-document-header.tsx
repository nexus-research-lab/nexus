/**
 * INPUT: 当前记忆文档、编辑状态、实时写入状态与操作。
 * OUTPUT: 可读摘要标题、更新时间和真实编辑/删除动作。
 * POS: 记忆正文唯一 Header；内部路径只保留为悬停诊断。
 */
import {
  ArrowLeft,
  LoaderCircle,
  Pencil,
  Save,
  Trash2,
  X,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import type { MemoryDocument } from "@/types/memory/memory";

import { formatMemoryModifiedTime } from "../memory-utils";
import { getMemoryDocumentDisplayTitle } from "../catalog/memory-catalog-presentation";
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
  deleteBusy: boolean;
  deleting: boolean;
  document: MemoryDocument;
  locale: string;
  onBack: () => void;
  onDelete: () => void;
  runtimeWriting: boolean;
}

export function MemoryDocumentHeader({
  controller,
  deleteBusy,
  deleting,
  document,
  locale,
  onBack,
  onDelete,
  runtimeWriting,
}: MemoryDocumentHeaderProps) {
  const { t } = useI18n();
  const model = buildMemoryDocumentHeaderModel({
    deleteBusy,
    deleting,
    dirty: controller.dirty,
    documentKind: document.kind,
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
              className="truncate text-base font-semibold text-(--text-strong)"
              title={document.path}
            >
              {getMemoryDocumentDisplayTitle(document)}
            </h2>
            {runtimeWriting ? <MemoryRuntimeWritingStatus /> : null}
          </div>
          <div className="mt-0.5 text-xs text-(--text-soft)">
            {formatMemoryModifiedTime(document.modified_at, locale)}
          </div>
        </div>
        <MemoryHeaderActions
          action={model.action}
          controller={controller}
          deleteAction={model.deleteAction}
          onDelete={onDelete}
        />
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
  deleteAction,
  onDelete,
}: {
  action: MemoryDocumentHeaderAction;
  controller: MemoryDocumentHeaderController;
  deleteAction: ReturnType<typeof buildMemoryDocumentHeaderModel>["deleteAction"];
  onDelete: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className="flex shrink-0 items-center gap-1.5">
      {action.kind === "edit" ? (
        <UiIconButton
          aria-label={t("common.edit")}
          disabled={action.disabled}
          onClick={controller.startEditing}
          size="md"
          title={t("common.edit")}
          variant="ghost"
        >
          <Pencil className="h-4 w-4" />
        </UiIconButton>
      ) : (
        <MemoryEditingActions action={action} controller={controller} />
      )}
      {deleteAction.visible ? (
        <UiIconButton
          aria-label={t("capability.memory_delete")}
          disabled={deleteAction.disabled}
          onClick={onDelete}
          size="md"
          title={t("capability.memory_delete")}
          tone="danger"
          variant="ghost"
        >
          {deleteAction.deleting ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : (
            <Trash2 className="h-4 w-4" />
          )}
        </UiIconButton>
      ) : null}
    </div>
  );
}

function MemoryEditingActions({
  action,
  controller,
}: {
  action: Extract<MemoryDocumentHeaderAction, { kind: "editing" }>;
  controller: MemoryDocumentHeaderController;
}) {
  const { t } = useI18n();
  const SaveIcon = action.saving ? LoaderCircle : Save;
  return (
    <>
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
    </>
  );
}
