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
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import type { MemoryDocument } from "@/types/memory/memory";

import {
  formatMemoryFileSize,
  formatMemoryModifiedTime,
} from "../memory-utils";
import {
  buildMemoryDocumentHeaderModel,
  type MemoryDocumentHeaderAction,
  type MemoryDocumentHeaderBadge,
  type MemoryDocumentHeaderBadgeKind,
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
    document,
    editing: controller.editing,
    isSaving: controller.isSaving,
    runtimeWriting,
  });
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
          {model.badges.map((badge) => (
            <MemoryHeaderBadge badge={badge} key={badge.kind} />
          ))}
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
      <MemoryHeaderActions action={model.action} controller={controller} />
    </div>
  );
}

const HEADER_BADGE_VIEW_BY_KIND: Readonly<Record<
  MemoryDocumentHeaderBadgeKind,
  {
    className: string;
    icon: typeof Link2;
    iconClassName: string;
  }
>> = {
  indexed: {
    className: "text-emerald-600 dark:text-emerald-400",
    icon: Link2,
    iconClassName: "h-3 w-3",
  },
  runtime_writing: {
    className: "text-(--primary)",
    icon: LoaderCircle,
    iconClassName: "h-3 w-3 animate-spin",
  },
};

function MemoryHeaderBadge({ badge }: { badge: MemoryDocumentHeaderBadge }) {
  const { t } = useI18n();
  const view = HEADER_BADGE_VIEW_BY_KIND[badge.kind];
  const Icon = view.icon;
  return (
    <span className={`inline-flex shrink-0 items-center gap-1 text-[10.5px] font-medium ${view.className}`}>
      <Icon className={view.iconClassName} />
      {t(badge.labelKey)}
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
