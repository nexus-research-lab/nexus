"use client";

/**
 * INPUT: 当前 Session 已授权目录、选择/移除命令与忙碌状态。
 * OUTPUT: 共享目录 Chip/范围 Badge、微型动作和读取失败反馈，阻塞 mutation 时明确禁用。
 * POS: Composer 本机目录纯视图；不判断目录授权或持久化结果。
 */

import { Folder, Laptop, Plus } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiRemovableChip } from "@/shared/ui/form/removable-chip";
import { UiBadge } from "@/shared/ui/display/badge";

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
  const actionsBlocked = disabled || controller.saving || controller.failure?.blocksMutation === true;
  return (
    <div className="mb-2 space-y-2">
      {controller.directories.length > 0 ? (
        <div
          aria-label={t("composer.local_directories_label")}
          className="scrollbar-hide flex min-h-8 min-w-0 flex-nowrap items-center gap-1.5 overflow-x-auto overflow-y-hidden overscroll-x-contain px-1"
          role="group"
        >
          <UiBadge className="shrink-0" size="sm">
            <Laptop aria-hidden className="h-3.5 w-3.5 text-(--icon-muted)" />
            {t("composer.local_directory_scope")}
          </UiBadge>
          {controller.directories.map((directory) => {
            const name = localDirectoryName(directory);
            return (
              <UiRemovableChip
                disabled={actionsBlocked}
                key={directory}
                onRemove={() => void controller.removeDirectory(directory)}
                removeLabel={t("composer.remove_local_directory", { name })}
                size="xs"
              >
                <span className="inline-flex min-w-0 items-center gap-1.5" title={directory}>
                  <Folder aria-hidden className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
                  <span className="max-w-[180px] truncate">{name}</span>
                </span>
              </UiRemovableChip>
            );
          })}
          <UiIconButton
            aria-label={t("composer.add_local_directory")}
            className="shrink-0"
            disabled={actionsBlocked || controller.loading}
            onClick={() => void controller.chooseDirectory()}
            size="md"
            tooltip={t("composer.add_local_directory")}
            variant="surface"
          >
            <Plus className="h-4 w-4" />
          </UiIconButton>
        </div>
      ) : null}
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
