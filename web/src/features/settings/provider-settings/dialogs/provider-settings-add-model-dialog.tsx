// INPUT: 当前 Provider 的手工模型草稿、启用选择和添加命令状态。
// OUTPUT: Model ID 与一个启用开关组成的 plain 表单弹窗。
// POS: Provider 手工模型入口，不重复解释后续模型配置能力。
import { useEffect, useRef } from "react";
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
  const modelInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      modelInputRef.current?.focus();
    }
  }, [isOpen]);

  if (!isOpen) {
    return null;
  }

  const isAdding = pendingAction?.kind === "add-model";

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialog"
        labelledBy="provider-add-model-title"
        onClose={onClose}
      >
        <UiDialogFormShell
          onSubmit={(event) => {
            event.preventDefault();
            onAdd();
          }}
          size="md"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("settings.providers.add_model_title")}
            titleId="provider-add-model-title"
          />
          <UiDialogBody className="space-y-4 px-5">
            <UiField
              description={t("settings.providers.add_model_description")}
              htmlFor="provider-model-id"
              label={t("settings.providers.model_id")}
              required
            >
              <UiInput
                aria-label={t("settings.providers.model_id")}
                autoCapitalize="off"
                autoCorrect="off"
                controlSize="md"
                className={getUiTypographyClassName({ role: "code", tone: "strong" })}
                id="provider-model-id"
                ref={modelInputRef}
                onChange={(event) => setManualModelId(event.target.value)}
                placeholder={manualModelPlaceholder}
                required
                spellCheck={false}
                type="text"
                value={manualModelId}
              />
            </UiField>
            <div className="flex items-center justify-between gap-3 border-t border-(--divider-subtle-color) py-3">
              <div className="min-w-0">
                <div className={getUiTypographyClassName({ role: "control", tone: "strong", weight: "semibold" })}>
                  {t("settings.providers.enable_after_add")}
                </div>
                <div className={cn("mt-0.5", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>
                  {t("settings.providers.enable_after_add_description")}
                </div>
              </div>
              <GlassSwitch
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
              disabled={isAdding || !selectedCanManage}
              tone="primary"
              type="submit"
              variant="solid"
            >
              {isAdding ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
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
