/**
 * INPUT: 当前开放状态、Loop 目录选择动作与关闭动作。
 * OUTPUT: 以搜索和分类筛选为主的紧凑 Loop 选择器。
 * POS: Composer 能力菜单中的 Loop 选择边界；不解释 Goal 执行状态。
 */
"use client";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { LoopCatalogItem } from "@/types/capability/loop";

import { LoopPickerContent } from "./loop-picker-content";
import { useLoopPickerController } from "./use-loop-picker-controller";

interface LoopPickerDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSelect: (loop: LoopCatalogItem) => void | Promise<void>;
}

export function LoopPickerDialog(props: LoopPickerDialogProps) {
  if (!props.isOpen) {
    return null;
  }
  return <OpenLoopPickerDialog {...props} />;
}

function OpenLoopPickerDialog({
  onClose,
  onSelect,
}: LoopPickerDialogProps) {
  const { t } = useI18n();
  const controller = useLoopPickerController({ onClose, onSelect });
  return (
    <UiDialogPortal>
      <UiDialogBackdrop onClose={onClose}>
        <UiDialogShell
          size="lg"
          style={{ maxHeight: "min(640px, calc(100vh - 96px))" }}
        >
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("composer.loop_picker_title")}
          />
          <UiDialogBody className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="flex shrink-0 flex-col gap-2 sm:flex-row">
              <UiSearchInput
                ref={controller.refs.searchInputRef}
                aria-label={t("composer.loop_search_placeholder")}
                className="min-w-0 flex-1"
                inputClassName="text-sm"
                onChange={controller.actions.setQuery}
                placeholder={t("composer.loop_search_placeholder")}
                value={controller.state.query}
              />
              <UiSelectMenu
                ariaLabel={t("capability.loops_filter_aria")}
                className="sm:w-[180px]"
                onChange={controller.actions.setCategory}
                options={controller.state.categoryOptions}
                size="sm"
                surface="dialog"
                value={controller.state.category}
              />
            </div>
            <LoopPickerContent
              busySlug={controller.state.busySlug}
              error={controller.state.error}
              isLoading={controller.state.isLoading}
              loops={controller.state.filteredLoops}
              onSelect={controller.actions.selectLoop}
            />
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
