// INPUT: 当前 Provider 的手工模型草稿、启用选择和添加命令状态。
// OUTPUT: 实例级 Model ID 字段与具名/关联说明的启用开关，复用 Dialog 焦点和 Field 技术文本。
// POS: Provider 手工模型入口，不重复解释后续模型配置能力。
import { useId, useRef } from "react";
import { Loader2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type { ProviderPendingAction } from "../actions/use-provider-command";

interface ProviderAddModelDialogProps {
  isOpen: boolean;
  manualModelEnabled: boolean;
  manualModelId: string;
  manualModelPlaceholder: string;
  onAdd: () => void;
  onClose: () => void;
  pendingAction: ProviderPendingAction | null;
  selectedCanManage: boolean;
  setManualModelEnabled: (enabled: boolean) => void;
  setManualModelId: (modelId: string) => void;
}

export function ProviderAddModelDialog({
  isOpen,
  manualModelEnabled,
  manualModelId,
  manualModelPlaceholder,
  onAdd,
  onClose,
  pendingAction,
  selectedCanManage,
  setManualModelEnabled,
  setManualModelId,
}: ProviderAddModelDialogProps) {
  const { t } = useI18n();
  const dialogId = useId();
  const modelInputRef = useRef<HTMLInputElement>(null);

  if (!isOpen) {
    return null;
  }

  const isAdding = pendingAction?.kind === "add-model";

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        initialFocusRef={modelInputRef}
        layer="dialog"
        labelledBy={`${dialogId}-title`}
        onClose={onClose}
      >
        <UiDialogFormShell
          onSubmit={(event) => {
            event.preventDefault();
            onAdd();
          }}
          size="md"
          viewport="adaptiveMax"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("settings.providers.add_model_title")}
            titleId={`${dialogId}-title`}
          />
          <UiDialogBody className="space-y-4 px-5" scrollable>
            <UiField
              description={t("settings.providers.add_model_description")}
              htmlFor={`${dialogId}-model`}
              label={t("settings.providers.model_id")}
              required
            >
              <UiInput
                aria-label={t("settings.providers.model_id")}
                autoCapitalize="off"
                autoCorrect="off"
                controlSize="md"
                id={`${dialogId}-model`}
                ref={modelInputRef}
                onChange={(event) => setManualModelId(event.target.value)}
                placeholder={manualModelPlaceholder}
                required
                spellCheck={false}
                textRole="code"
                type="text"
                value={manualModelId}
              />
            </UiField>
            <div className="flex items-center justify-between gap-3 border-t border-(--divider-subtle-color) py-3">
              <div className="min-w-0">
                <div className={getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" })}>
                  {t("settings.providers.enable_after_add")}
                </div>
                <div id={`${dialogId}-enable-description`} className={cn("mt-0.5", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
                  {t("settings.providers.enable_after_add_description")}
                </div>
              </div>
              <GlassSwitch
                aria-describedby={`${dialogId}-enable-description`}
                aria-label={t("settings.providers.enable_after_add")}
                checked={manualModelEnabled}
                size="xs"
                onChange={setManualModelEnabled}
              />
            </div>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton
              onClick={onClose}
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              aria-busy={isAdding}
              disabled={isAdding || !selectedCanManage}
              tone="primary"
              type="submit"
              variant="solid"
            >
              {isAdding ? (
                <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
              ) : null}
              {manualModelEnabled
                ? t("settings.providers.add_and_enable")
                : t("settings.providers.add")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
