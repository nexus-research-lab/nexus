// INPUT: 单个 Provider 模型的能力、窗口、输出限制和 JSON Options 草稿。
// OUTPUT: 模型身份、双列能力、额度与折叠参数；只读或忙碌时统一禁用编辑与保存。
// POS: Provider 模型覆写入口，不把每项能力包装成图标卡片。
import { useId, type Dispatch, type SetStateAction } from "react";
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
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiField, UiInput, UiTextarea } from "@/shared/ui/form/form-control";
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

const CAPABILITY_FIELDS = [
  { key: "vision", label: "settings.providers.capability_vision" },
  { key: "image_output", label: "settings.providers.capability_image_output" },
  { key: "tool_calling", label: "settings.providers.capability_tool_calling" },
  { key: "reasoning", label: "settings.providers.capability_reasoning" },
  { key: "embedding", label: "settings.providers.capability_embedding" },
] as const;

export function ProviderModelOptionsDialog({
  modelOptions,
  onClose,
  onSave,
  pendingAction,
  selectedCanManage,
  setModelOptions,
}: ProviderModelOptionsDialogProps) {
  const { t } = useI18n();
  const dialogId = useId();

  if (!modelOptions) {
    return null;
  }

  const controlsDisabled = pendingAction !== null || !selectedCanManage;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        layer="dialog"
        labelledBy={`${dialogId}-title`}
        onClose={onClose}
      >
        <UiDialogShell size="md" viewport="adaptiveMax">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("settings.providers.model_options")}
            titleId={`${dialogId}-title`}
            subtitle={<code className="break-all">{modelOptions.model.model_id}</code>}
          />
          <UiDialogBody className="space-y-5 px-5" scrollable>
            <section className="space-y-2.5">
              <div>
                <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
                  {t("settings.providers.model_capabilities")}
                </h3>
                <p className={cn("mt-0.5", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
                  {t("settings.providers.model_capabilities_description")}
                </p>
              </div>
              <div className="grid gap-x-6 gap-y-1 sm:grid-cols-2">
                {CAPABILITY_FIELDS.map(({ key, label }) => (
                  <CapabilitySwitch
                    checked={!!modelOptions.capabilities[key]}
                    disabled={controlsDisabled}
                    key={key}
                    label={t(label)}
                    onChange={(checked) => setModelOptions((current) => current ? ({
                      ...current,
                      capabilities: { ...current.capabilities, [key]: checked },
                    }) : current)}
                  />
                ))}
              </div>
            </section>

            <section className="grid gap-3 sm:grid-cols-2">
              <UiField htmlFor={`${dialogId}-context`} label={t("settings.providers.context_window")}>
                <UiInput
                  controlSize="md"
                  disabled={controlsDisabled}
                  id={`${dialogId}-context`}
                  inputMode="numeric"
                  onChange={(event) => setModelOptions((current) => current ? ({ ...current, context_window: event.target.value }) : current)}
                  placeholder="auto"
                  value={modelOptions.context_window}
                />
              </UiField>
              <UiField htmlFor={`${dialogId}-output`} label={t("settings.providers.max_output_tokens")}>
                <UiInput
                  controlSize="md"
                  disabled={controlsDisabled}
                  id={`${dialogId}-output`}
                  inputMode="numeric"
                  onChange={(event) => setModelOptions((current) => current ? ({ ...current, max_output_tokens: event.target.value }) : current)}
                  placeholder="auto"
                  value={modelOptions.max_output_tokens}
                />
              </UiField>
            </section>

            <UiDisclosure
              key={modelOptions.model.id}
              label={t("settings.providers.provider_options_json")}
              variant="section"
              defaultOpen={modelOptions.provider_options_text.trim() !== "" && modelOptions.provider_options_text.trim() !== "{}"}
            >
              <UiTextarea
                aria-label={t("settings.providers.provider_options_json")}
                controlSize="md"
                disabled={controlsDisabled}
                id={`${dialogId}-options`}
                onChange={(event) => setModelOptions((current) => current ? ({ ...current, provider_options_text: event.target.value }) : current)}
                spellCheck={false}
                textRole="code"
                value={modelOptions.provider_options_text}
              />
            </UiDisclosure>
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
              aria-busy={pendingAction?.kind === "save-model-options"}
              disabled={controlsDisabled}
              onClick={onSave}
              tone="primary"
              type="button"
              variant="solid"
            >
              {pendingAction?.kind === "save-model-options" ? (
                <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
              ) : null}
              {pendingAction?.kind === "save-model-options" ? t("common.saving") : t("common.save")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
