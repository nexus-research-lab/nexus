"use client";

import { CircleAlert, Folder, Laptop, Plus, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

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
    || (controller.directories.length === 0 && !controller.error)
  ) {
    return null;
  }
  return (
    <div
      aria-label={t("composer.local_directories_label")}
      className="mb-2 flex min-h-8 flex-wrap items-center gap-1.5 px-1"
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
            className="radius-control-sm group flex h-8 min-w-0 items-center gap-1.5 border border-(--divider-subtle-color) bg-(--surface-raised-background) px-2.5 text-xs text-(--text-default)"
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
      {controller.error ? (
        <span
          aria-live="polite"
          className="radius-control-sm inline-flex min-h-8 max-w-full items-center gap-1.5 bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-2.5 text-xs text-(--destructive)"
          role="status"
        >
          <CircleAlert className="h-3.5 w-3.5 shrink-0" />
          <span className="truncate">{controller.error}</span>
        </span>
      ) : null}
    </div>
  );
}

function localDirectoryName(directory: string): string {
  const normalized = directory.replace(/[\\/]+$/, "");
  return normalized.split(/[\\/]/).at(-1) || directory;
}
