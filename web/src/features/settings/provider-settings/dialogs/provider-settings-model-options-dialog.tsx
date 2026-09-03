// INPUT: 单个 Provider 模型的能力、窗口、输出限制和 JSON Options 草稿。
// OUTPUT: 目标模型明确、设置行克制的 plain 配置弹窗。
// POS: Provider 模型覆写入口，不把每项能力包装成图标卡片。
import { type Dispatch, type SetStateAction } from "react";
import { Loader2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiInput, UiTextarea } from "@/shared/ui/form/form-control";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { CapabilitySwitch } from "../components/provider-settings-capability-switch";
import type { ProviderPendingAction } from "../actions/use-provider-command";
import type { ModelOptionsState } from "../model/provider-settings-types";

interface ProviderModelOptionsDialogProps {
  modelOptions: ModelOptionsState | null;
  onClose: () => void;
  onSave: () => void;
  pendingAction: ProviderPendingAction | null;
  selectedCanManage: boolean;
  setModelOptions: Dispatch<SetStateAction<ModelOptionsState | null>>;
}

export function ProviderModelOptionsDialog({
  modelOptions,
  onClose,
  onSave,
  pendingAction,
  selectedCanManage,
  setModelOptions,
}: ProviderModelOptionsDialogProps) {
  const { t } = useI18n();

  if (!modelOptions) {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialog"
        labelledBy="provider-model-options-title"
        onClose={onClose}
      >
        <UiDialogShell size="lg">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("settings.providers.model_options")}
            titleId="provider-model-options-title"
          />
          <UiDialogBody className="space-y-5 px-5" scrollable>
            <code className={cn("block truncate", getUiTypographyClassName({ role: "code", tone: "muted" }))}>
              {modelOptions.model.model_id}
            </code>
            <section className="space-y-2.5">
              <div>
                <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
                  {t("settings.providers.model_capabilities")}
                </h3>
                <p className={cn("mt-0.5", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>
                  {t("settings.providers.model_capabilities_description")}
                </p>
              </div>
              <div className="divide-y divide-(--divider-subtle-color) border-y border-(--divider-subtle-color)">
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.vision}
                  label={t("settings.providers.capability_vision")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, vision: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.image_output}
                  label={t("settings.providers.capability_image_output")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, image_output: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.tool_calling}
                  label={t("settings.providers.capability_tool_calling")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, tool_calling: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.reasoning}
                  label={t("settings.providers.capability_reasoning")}
                  onChange={(checked) => setModelOptions((current) => current ? ({
                    ...current,
                    capabilities: { ...current.capabilities, reasoning: checked },
                  }) : current)}
                />
                <CapabilitySwitch
                  checked={!!modelOptions.capabilities.embedding}
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
                <span className={getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" })}>
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
                <span className={getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" })}>
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
              <span className={getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" })}>
                {t("settings.providers.provider_options_json")}
              </span>
              <UiTextarea
                className={cn("min-h-28", getUiTypographyClassName({ role: "code", tone: "strong" }))}
                controlSize="md"
                onChange={(event) => setModelOptions((current) => current ? ({ ...current, provider_options_text: event.target.value }) : current)}
                spellCheck={false}
                value={modelOptions.provider_options_text}
              />
            </label>
          </UiDialogBody>
          <UiDialogFooter appearance="plain" className="gap-2">
            <UiButton
              onClick={onClose}
              size="sm"
              type="button"
              variant="surface"
            >
              {t("common.cancel")}
            </UiButton>
            <UiButton
              disabled={pendingAction?.kind === "save-model-options" || !selectedCanManage}
              onClick={onSave}
              size="sm"
              tone="primary"
              type="button"
              variant="solid"
            >
              {pendingAction?.kind === "save-model-options" ? (
                <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
              ) : t("common.save")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
