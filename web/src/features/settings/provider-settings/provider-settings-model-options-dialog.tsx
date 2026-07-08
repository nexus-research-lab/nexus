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
  modelOptions: ModelOptionsState | null;
  onClose: () => void;
  onSave: () => void;
  pendingAction: string | null;
  selectedCanManage: boolean;
  setModelOptions: Dispatch<SetStateAction<ModelOptionsState | null>>;
}

export function ProviderModelOptionsDialog({
  modelOptions: modelOptions,
  onClose: onClose,
  onSave: onSave,
  pendingAction: pendingAction,
  selectedCanManage: selectedCanManage,
  setModelOptions: setModelOptions,
}: ProviderModelOptionsDialogProps) {
  const { t } = useI18n();

  if (!modelOptions) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[9999]"
        labelledBy="provider-model-options-title"
        onClose={onClose}
      >
        <UiDialogShell className="max-w-[620px]" size="lg">
          <UiDialogHeader
            icon={<SlidersHorizontal className="h-4.5 w-4.5" />}
            iconClassName="rounded-[12px]"
            onClose={onClose}
            subtitle={(
              <span className="inline-flex min-w-0 items-center gap-1.5">
                <span>{t("settings.providers.model_options_subtitle")}</span>
                <code className="max-w-[260px] truncate rounded-[7px] bg-(--surface-muted-background) px-1.5 py-0.5 font-mono text-[11px] text-(--text-default)">
                  {modelOptions.model.model_id}
                </code>
              </span>
            )}
            title={t("settings.providers.model_options")}
            titleId="provider-model-options-title"
          />
          <UiDialogBody className="space-y-5" scrollable>
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
                  checked={!!modelOptions.capabilities.vision}
                  icon={<Eye className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_vision")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, vision: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.image_output}
                  icon={<Image className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_image_output")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, image_output: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.tool_calling}
                  icon={<Wrench className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_tool_calling")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, tool_calling: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.reasoning}
                  icon={<Brain className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_reasoning")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, reasoning: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.embedding}
                  icon={<Database className="h-3.5 w-3.5" />}
                  label={t("settings.providers.capability_embedding")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
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
                  controlSize="sm"
                  inputMode="numeric"
                  onChange={(event) => setModelOptions((current) => current ? ({ ...current, context_window: event.target.value }) : current)}
                  placeholder="auto"
                  value={modelOptions.context_window}
                />
              </label>
              <label className="space-y-1.5">
                <span className="text-[12px] font-medium text-(--text-muted)">
                  {t("settings.providers.max_output_tokens")}
                </span>
                <UiInput
                  controlSize="sm"
                  inputMode="numeric"
                  onChange={(event) => setModelOptions((current) => current ? ({ ...current, max_output_tokens: event.target.value }) : current)}
                  placeholder="auto"
                  value={modelOptions.max_output_tokens}
                />
              </label>
            </section>

            <label className="block space-y-1.5">
              <span className="text-[12px] font-medium text-(--text-muted)">
                {t("settings.providers.provider_options_json")}
              </span>
              <UiTextarea
                className="min-h-28 font-mono text-[12px] leading-5"
                controlSize="md"
                onChange={(event) => setModelOptions((current) => current ? ({ ...current, provider_options_text: event.target.value }) : current)}
                spellCheck={false}
                value={modelOptions.provider_options_text}
              />
            </label>
          </UiDialogBody>
          <UiDialogFooter className="gap-2">
            <UiButton
              onClick={onClose}
              size="sm"
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={pendingAction?.startsWith("options:") || !selectedCanManage}
              onClick={onSave}
              size="sm"
              tone="primary"
              type="button"
              variant="solid"
            >
              {pendingAction?.startsWith("options:") ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : t("common.save")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
