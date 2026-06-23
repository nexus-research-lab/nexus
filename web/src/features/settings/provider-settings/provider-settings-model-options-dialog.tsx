import { type Dispatch, type SetStateAction } from "react";
import { Brain, Database, Eye, Image, Loader2, SlidersHorizontal, Wrench } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiInput, UiTextarea } from "@/shared/ui/form-control";

import { CapabilitySwitch } from "./provider-settings-capability-switch";
import type { ModelOptionsState } from "./provider-settings-model";

interface ProviderModelOptionsDialogProps {
  model_options: ModelOptionsState | null;
  on_close: () => void;
  on_save: () => void;
  pending_action: string | null;
  selected_can_manage: boolean;
  set_model_options: Dispatch<SetStateAction<ModelOptionsState | null>>;
}

export function ProviderModelOptionsDialog({
  model_options,
  on_close,
  on_save,
  pending_action,
  selected_can_manage,
  set_model_options,
}: ProviderModelOptionsDialogProps) {
  const { t } = useI18n();

  if (!model_options) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        class_name="z-[9999]"
        labelled_by="provider-model-options-title"
        on_close={on_close}
      >
        <UiDialogShell class_name="max-w-[620px]" size="lg">
          <UiDialogHeader
            icon={<SlidersHorizontal className="h-4.5 w-4.5" />}
            icon_class_name="rounded-[12px]"
            on_close={on_close}
            subtitle={(
              <span className="inline-flex min-w-0 items-center gap-1.5">
                <span>{t("settings.providers.model_options_subtitle")}</span>
                <code className="max-w-[260px] truncate rounded-[7px] bg-(--surface-muted-background) px-1.5 py-0.5 font-mono text-[11px] text-(--text-default)">
                  {model_options.model.model_id}
                </code>
              </span>
            )}
            title={t("settings.providers.model_options")}
            title_id="provider-model-options-title"
          />
          <UiDialogBody class_name="space-y-5" scrollable>
            <section className="space-y-2.5">
              <div>
                <h3 className="text-[13px] font-semibold text-(--text-strong)">
                  {t("settings.providers.model_capabilities")}
                </h3>
                <p className="mt-0.5 text-[11px] leading-4 text-(--text-muted)">
                  {t("settings.providers.model_capabilities_description")}
                </p>
              </div>
              <div className="grid gap-2.5 md:grid-cols-2">
                <CapabilitySwitch
                  checked={!!model_options.capabilities.vision}
                  icon={<Eye className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_vision")}
                  on_change={(checked) => set_model_options((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, vision: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!model_options.capabilities.image_output}
                  icon={<Image className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_image_output")}
                  on_change={(checked) => set_model_options((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, image_output: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!model_options.capabilities.tool_calling}
                  icon={<Wrench className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_tool_calling")}
                  on_change={(checked) => set_model_options((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, tool_calling: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!model_options.capabilities.reasoning}
                  icon={<Brain className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_reasoning")}
                  on_change={(checked) => set_model_options((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, reasoning: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!model_options.capabilities.embedding}
                  icon={<Database className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_embedding")}
                  on_change={(checked) => set_model_options((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, embedding: checked },
                  }) : current)}
                />
              </div>
            </section>

            <section className="grid gap-3 md:grid-cols-2">
              <label className="space-y-1.5">
                <span className="text-[12px] font-medium text-(--text-muted)">
                  {t("settings.providers.context_window")}
                </span>
                <UiInput
                  control_size="sm"
                  inputMode="numeric"
                  onChange={(event) => set_model_options((current) => current ? ({ ...current, context_window: event.target.value }) : current)}
                  placeholder="auto"
                  value={model_options.context_window}
                />
              </label>
              <label className="space-y-1.5">
                <span className="text-[12px] font-medium text-(--text-muted)">
                  {t("settings.providers.max_output_tokens")}
                </span>
                <UiInput
                  control_size="sm"
                  inputMode="numeric"
                  onChange={(event) => set_model_options((current) => current ? ({ ...current, max_output_tokens: event.target.value }) : current)}
                  placeholder="auto"
                  value={model_options.max_output_tokens}
                />
              </label>
            </section>

            <label className="block space-y-1.5">
              <span className="text-[12px] font-medium text-(--text-muted)">
                {t("settings.providers.provider_options_json")}
              </span>
              <UiTextarea
                class_name="min-h-28 font-mono text-[12px] leading-5"
                control_size="md"
                onChange={(event) => set_model_options((current) => current ? ({ ...current, provider_options_text: event.target.value }) : current)}
                spellCheck={false}
                value={model_options.provider_options_text}
              />
            </label>
          </UiDialogBody>
          <UiDialogFooter class_name="gap-2">
            <UiButton
              onClick={on_close}
              size="sm"
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={pending_action?.startsWith("options:") || !selected_can_manage}
              onClick={on_save}
              size="sm"
              tone="primary"
              type="button"
              variant="solid"
            >
              {pending_action?.startsWith("options:") ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : t("common.save")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
