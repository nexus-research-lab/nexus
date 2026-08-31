// INPUT: 当前模型选择、已加载 Provider 选项与读取失败状态。
// OUTPUT: 不丢当前选择、说明读取影响和恢复路径的模型选择控件。
// POS: Agent 身份表单纯视图；不展示 Provider 原始错误，也不发起保存。
import { useCallback, useMemo } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { AgentProvider } from "@/types/agent/agent";
import {
  formatProviderLabel,
  formatProviderOptionLabel,
  type ProviderOption,
} from "@/types/capability/provider";

import {
  IDENTITY_FIELD_LABEL_CLASS_NAMES,
  type AgentIdentityVariant,
} from "./identity-layout";

interface ModelSelectorLayout {
  buttonClassName: string;
  className: string;
  errorClassName: string;
  size?: "sm";
}

const MODEL_SELECTOR_LAYOUTS: Record<
  AgentIdentityVariant,
  ModelSelectorLayout
> = {
  dialog: {
    buttonClassName: "h-auto min-h-11 py-2.5",
    className: "h-auto min-h-11",
    errorClassName: "mt-2 text-xs text-rose-500",
  },
  inline: {
    buttonClassName: "h-auto min-h-9 py-2",
    className: "h-auto min-h-9",
    errorClassName: "text-xs text-rose-500",
    size: "sm",
  },
};

interface IdentityModelSelectorProps {
  defaultModel: string;
  defaultProvider: AgentProvider;
  error: string | null;
  lockedToDefault?: boolean;
  loading: boolean;
  model: string;
  onModelChange: (value: string) => void;
  onProviderChange: (value: AgentProvider) => void;
  options: ProviderOption[];
  provider: AgentProvider;
  variant: AgentIdentityVariant;
}

interface ModelSelection {
  model: string;
  provider: AgentProvider;
}

const DEFAULT_MODEL_SELECTION: ModelSelection = { model: "", provider: "" };

export function IdentityModelSelector({
  defaultModel,
  defaultProvider,
  error,
  lockedToDefault = false,
  loading,
  model,
  onModelChange,
  onProviderChange,
  options,
  provider,
  variant,
}: IdentityModelSelectorProps) {
  const { t } = useI18n();
  const layout = MODEL_SELECTOR_LAYOUTS[variant];
  const labelClassName = IDENTITY_FIELD_LABEL_CLASS_NAMES[variant];
  const selectedValue = encodeModelSelection({ model, provider });
  const defaultLabel = defaultProvider && defaultModel
    ? t("agent_options.identity.follow_default_provider_named", {
      name: `${formatProviderLabel(defaultProvider)} / ${defaultModel}`,
    })
    : t("agent_options.identity.follow_default_provider");
  const selectedUnavailable = !lockedToDefault
    && Boolean(provider.trim() && model.trim())
    && !options.some((providerOption) => (
      providerOption.provider.trim() === provider.trim()
      && providerOption.models.some((modelOption) => modelOption.model_id.trim() === model.trim())
    ));
  const selectOptions = useMemo(() => [
    { label: defaultLabel, value: "" },
    ...(selectedUnavailable ? [{
      label: t("agent_options.identity.model_temporarily_unavailable_option", {
        name: `${formatProviderLabel(provider)} / ${model}`,
      }),
      value: selectedValue,
    }] : []),
    ...options.flatMap((providerOption) => {
      const providerLabel = formatProviderOptionLabel(
        providerOption,
        t("settings.providers.subscription_badge"),
      );
      return providerOption.models.map((modelOption) => ({
        label: `${providerLabel} / ${modelOption.display_name || modelOption.model_id}`,
        value: encodeModelSelection({
          model: modelOption.model_id,
          provider: providerOption.provider,
        }),
      }));
    }),
  ], [defaultLabel, model, options, provider, selectedUnavailable, selectedValue, t]);

  const handleChange = useCallback((value: string) => {
    const selection = decodeModelSelection(value);
    if (!selection) {
      return;
    }
    onProviderChange(selection.provider);
    onModelChange(selection.model);
  }, [onModelChange, onProviderChange]);

  const followDefault = useCallback(() => {
    onProviderChange(DEFAULT_MODEL_SELECTION.provider);
    onModelChange(DEFAULT_MODEL_SELECTION.model);
  }, [onModelChange, onProviderChange]);

  return (
    <div className="space-y-3">
      <label className={labelClassName}>
        {t("agent_options.identity.model")}
      </label>
      <UiSelectMenu
        allowLabelWrap
        ariaLabel={t("agent_options.identity.model")}
        buttonClassName={layout.buttonClassName}
        className={layout.className}
        disabled={lockedToDefault || (loading && options.length === 0)}
        menuMinWidth={460}
        onChange={handleChange}
        options={selectOptions}
        size={layout.size}
        surface="dialog"
        value={selectedValue}
      />
      {lockedToDefault ? (
        <p className="text-xs text-(--text-soft)">
          {t("agent_options.identity.main_model_hint")}
        </p>
      ) : selectedUnavailable ? (
        <div className="surface-radius-md flex flex-wrap items-center justify-between gap-2 border border-[color:color-mix(in_srgb,var(--warning)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] px-3 py-2">
          <p className="text-xs leading-5 text-(--warning)">
            {t("agent_options.identity.model_temporarily_unavailable")}
          </p>
          <UiButton
            className="shrink-0"
            onClick={followDefault}
            size="xs"
            type="button"
            variant="ghost"
          >
            {t("agent_options.identity.use_default_model")}
          </UiButton>
        </div>
      ) : error ? (
        <div
          aria-atomic="true"
          aria-live="polite"
          className={layout.errorClassName}
          role="status"
        >
          <p className="font-semibold">
            {t("agent_options.identity.provider_load_failed")}
          </p>
          <p className="mt-1 leading-5 text-(--text-muted)">
            {t("agent_options.identity.provider_load_failed_impact")}
          </p>
          <p className="mt-1 font-medium leading-5 text-(--text-default)">
            {t("agent_options.identity.provider_load_failed_next_step")}
          </p>
        </div>
      ) : null}
    </div>
  );
}

function encodeModelSelection({ model, provider }: ModelSelection): string {
  const normalizedProvider = provider.trim();
  const normalizedModel = model.trim();
  return normalizedProvider && normalizedModel
    ? JSON.stringify([normalizedProvider, normalizedModel])
    : "";
}

function decodeModelSelection(value: string): ModelSelection | null {
  if (!value) {
    return DEFAULT_MODEL_SELECTION;
  }

  try {
    const parsed = JSON.parse(value) as unknown;
    if (!isModelSelectionTuple(parsed)) {
      return null;
    }
    return {
      model: parsed[1].trim(),
      provider: parsed[0].trim(),
    };
  } catch {
    return null;
  }
}

function isModelSelectionTuple(value: unknown): value is [string, string] {
  return Array.isArray(value)
    && value.length === 2
    && value.every((item) => typeof item === "string");
}
