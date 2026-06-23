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

interface ProviderAddModelDialogProps {
  is_open: boolean;
  manual_model_enabled: boolean;
  manual_model_id: string;
  manual_model_placeholder: string;
  on_add: () => void;
  on_close: () => void;
  pending_action: string | null;
  selected_can_manage: boolean;
  set_manual_model_enabled: (enabled: boolean) => void;
  set_manual_model_id: (model_id: string) => void;
}

export function ProviderAddModelDialog({
  is_open,
  manual_model_enabled,
  manual_model_id,
  manual_model_placeholder,
  on_add,
  on_close,
  pending_action,
  selected_can_manage,
  set_manual_model_enabled,
  set_manual_model_id,
}: ProviderAddModelDialogProps) {
  const { t } = useI18n();

  if (!is_open) {
    return null;
  }

  const is_adding = pending_action?.startsWith("add-model:") ?? false;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9999]"
        labelled_by="provider-add-model-title"
        on_close={on_close}
      >
        <UiDialogFormShell
          class_name="max-w-[520px]"
          onSubmit={(event) => {
            event.preventDefault();
            on_add();
          }}
          size="md"
        >
          <UiDialogHeader
            icon={<ListPlus className="h-4.5 w-4.5" />}
            on_close={on_close}
            subtitle={t("settings.providers.add_model_subtitle")}
            title={t("settings.providers.add_model_title")}
            title_id="provider-add-model-title"
          />
          <UiDialogBody class_name="space-y-4">
            <UiField
              description={t("settings.providers.add_model_description")}
              label={t("settings.providers.model_id")}
            >
              <UiInput
                autoCapitalize="off"
                autoCorrect="off"
                autoFocus
                control_size="lg"
                class_name="font-mono"
                onChange={(event) => set_manual_model_id(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    on_add();
                  }
                }}
                placeholder={manual_model_placeholder}
                spellCheck={false}
                type="text"
                value={manual_model_id}
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
                checked={manual_model_enabled}
                size="xs"
                on_change={set_manual_model_enabled}
              />
            </div>
          </UiDialogBody>
          <UiDialogFooter>
            <UiButton
              onClick={on_close}
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={!manual_model_id.trim() || is_adding || !selected_can_manage}
              onClick={on_add}
              tone="primary"
              type="button"
              variant="solid"
            >
              {is_adding ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <ListPlus className="h-3.5 w-3.5" />}
              {manual_model_enabled
                ? t("settings.providers.add_and_enable")
                : t("settings.providers.add")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
