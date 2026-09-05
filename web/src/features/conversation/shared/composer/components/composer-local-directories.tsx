"use client";

/**
 * INPUT: 当前 Session 已授权目录、选择/移除命令与忙碌状态。
 * OUTPUT: Composer 内可横向收敛的目录范围 chip、共享微型动作与读取失败反馈。
 * POS: Composer 本机目录纯视图；不判断目录授权或持久化结果。
 */

import { Folder, Laptop, Plus, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
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
            className="radius-control-sm group flex h-8 min-w-0 shrink-0 items-center gap-1.5 border border-(--divider-subtle-color) bg-(--surface-panel-background) px-2.5 text-xs text-(--text-default)"
            key={directory}
            title={directory}
          >
            <Folder className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="max-w-[180px] truncate">{name}</span>
            <UiIconButton
              aria-label={t("composer.remove_local_directory", { name })}
              className="-mr-1 shrink-0"
              disabled={disabled || controller.saving}
              onClick={() => void controller.removeDirectory(directory)}
              size="2xs"
              tone="danger"
              tooltip={t("composer.remove_local_directory", { name })}
              variant="ghost"
            >
              <X className="h-3 w-3" />
            </UiIconButton>
          </div>
        );
      })}
      {controller.directories.length > 0 ? (
        <UiIconButton
          aria-label={t("composer.add_local_directory")}
          className="shrink-0"
          disabled={disabled || controller.loading || controller.saving}
          onClick={() => void controller.chooseDirectory()}
          size="md"
          tooltip={t("composer.add_local_directory")}
          variant="surface"
        >
          <Plus className="h-4 w-4" />
        </UiIconButton>
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
