"use client";

import { Folder, Laptop, Plus, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type { ComposerLocalDirectoriesController } from "../controller/use-composer-local-directories";

export function ComposerLocalDirectories({
  controller,
  disabled,
}: {
  controller: ComposerLocalDirectoriesController;
  disabled: boolean;
}) {
  const { t } = useI18n();
  if (
    !controller.available
    || (controller.directories.length === 0 && !controller.failure)
  ) {
    return null;
  }
  return (
    <div className="mb-2 space-y-2">
      <div
        aria-label={t("composer.local_directories_label")}
        className="scrollbar-hide flex min-h-8 min-w-0 flex-nowrap items-center gap-1.5 overflow-x-auto overflow-y-hidden overscroll-x-contain px-1"
      >
      {controller.directories.length > 0 ? (
        <span className="radius-control-sm inline-flex h-8 shrink-0 items-center gap-1.5 border border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) px-2.5 text-xs font-medium text-(--text-soft)">
          <Laptop className="h-3.5 w-3.5 text-(--icon-muted)" />
          {t("composer.local_directory_scope")}
        </span>
      ) : null}
      {controller.directories.map((directory) => {
        const name = localDirectoryName(directory);
        return (
          <div
            className="radius-control-sm group flex h-8 min-w-0 shrink-0 items-center gap-1.5 border border-(--divider-subtle-color) bg-(--surface-raised-background) px-2.5 text-xs text-(--text-default)"
            key={directory}
            title={directory}
          >
            <Folder className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="max-w-[180px] truncate">{name}</span>
            <button
              aria-label={t("composer.remove_local_directory", { name })}
              className="radius-control-xs -mr-1 inline-flex h-5 w-5 shrink-0 items-center justify-center text-(--icon-muted) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--destructive) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
              disabled={disabled || controller.saving}
              onClick={() => void controller.removeDirectory(directory)}
              type="button"
            >
              <X className="h-3 w-3" />
            </button>
          </div>
        );
      })}
      {controller.directories.length > 0 ? (
        <button
          aria-label={t("composer.add_local_directory")}
          className="radius-control-sm inline-flex h-8 w-8 shrink-0 items-center justify-center border border-(--divider-subtle-color) bg-transparent text-(--icon-muted) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
          disabled={disabled || controller.loading || controller.saving}
          onClick={() => void controller.chooseDirectory()}
          type="button"
        >
          <Plus className="h-4 w-4" />
        </button>
      ) : null}
      </div>
      {controller.failure ? (
        <UiResourceState
          impact={controller.failure.impact}
          primaryAction={{
            label: t("composer.local_directories_reload"),
            onClick: controller.reload,
          }}
          size="sm"
          state="error"
          title={t("composer.local_directories_failure_title")}
        />
      ) : null}
    </div>
  );
}

function localDirectoryName(directory: string): string {
  const normalized = directory.replace(/[\\/]+$/, "");
  return normalized.split(/[\\/]/).at(-1) || directory;
}
