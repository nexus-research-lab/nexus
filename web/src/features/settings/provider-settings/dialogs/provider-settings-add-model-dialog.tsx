import { useEffect, useRef } from "react";
import { ListPlus, Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiField, UiInput } from "@/shared/ui/form-control";
import { GlassSwitch } from "@/shared/ui/liquid-glass";

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
        className="z-[9999]"
        labelledBy="provider-add-model-title"
        onClose={onClose}
      >
        <UiDialogFormShell
          className="max-w-[520px]"
          onSubmit={(event) => {
            event.preventDefault();
            onAdd();
          }}
          size="md"
        >
          <UiDialogHeader
            icon={<ListPlus className="h-4.5 w-4.5" />}
            onClose={onClose}
            subtitle={t("settings.providers.add_model_subtitle")}
            title={t("settings.providers.add_model_title")}
            titleId="provider-add-model-title"
          />
          <UiDialogBody className="space-y-4">
            <UiField
              description={t("settings.providers.add_model_description")}
              label={t("settings.providers.model_id")}
            >
              <UiInput
                aria-label={t("settings.providers.model_id")}
                autoCapitalize="off"
                autoCorrect="off"
                controlSize="lg"
                className="font-mono"
                ref={modelInputRef}
                onChange={(event) => setManualModelId(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    onAdd();
                  }
                }}
                placeholder={manualModelPlaceholder}
                spellCheck={false}
                type="text"
                value={manualModelId}
              />
            </UiField>
            <div className="flex items-center justify-between gap-3 rounded-[14px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_76%,transparent)] px-3.5 py-3">
              <div className="min-w-0">
                <div className="text-[13px] font-semibold text-(--text-strong)">
                  {t("settings.providers.enable_after_add")}
                </div>
                <div className="mt-0.5 text-[11px] leading-4 text-(--text-muted)">
                  {t("settings.providers.enable_after_add_description")}
                </div>
              </div>
              <GlassSwitch
                checked={manualModelEnabled}
                size="xs"
                onChange={setManualModelEnabled}
              />
            </div>
          </UiDialogBody>
          <UiDialogFooter>
            <UiButton
              onClick={onClose}
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={!manualModelId.trim() || isAdding || !selectedCanManage}
              onClick={onAdd}
              tone="primary"
              type="button"
              variant="solid"
            >
              {isAdding ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ListPlus className="h-3.5 w-3.5" />}
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
