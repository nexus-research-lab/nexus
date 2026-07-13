import { useCallback, useMemo } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { AgentProvider } from "@/types/agent/agent";
import {
  formatProviderLabel,
  formatProviderOptionLabel,
  type ProviderOption,
} from "@/types/capability/provider";

import type { AgentIdentityVariant } from "./identity-layout";

interface ModelSelectorLayout {
  buttonClassName: string;
  className: string;
  errorClassName: string;
  labelClassName: string;
  size?: "sm";
}

const MODEL_SELECTOR_LAYOUTS: Record<
  AgentIdentityVariant,
  ModelSelectorLayout
> = {
  dialog: {
    buttonClassName: "h-auto min-h-10 py-2",
    className: "h-auto min-h-10",
    errorClassName: "mt-2 text-xs text-rose-500",
    labelClassName: "text-[11px] font-semibold text-(--text-muted)",
  },
  inline: {
    buttonClassName: "h-auto min-h-9 py-2",
    className: "h-auto min-h-9",
    errorClassName: "text-xs text-rose-500",
    labelClassName:
      "text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)",
    size: "sm",
  },
};

interface IdentityModelSelectorProps {
  defaultModel: string;
  defaultProvider: AgentProvider;
  error: string | null;
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
  const selectedValue = encodeModelSelection({ model, provider });
  const defaultLabel = defaultProvider && defaultModel
    ? t("agent_options.identity.follow_default_provider_named", {
      name: `${formatProviderLabel(defaultProvider)} / ${defaultModel}`,
    })
    : t("agent_options.identity.follow_default_provider");
  const selectOptions = useMemo(() => [
    { label: defaultLabel, value: "" },
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
  ], [defaultLabel, options, t]);

  const handleChange = useCallback((value: string) => {
    const selection = decodeModelSelection(value);
    if (!selection) {
      return;
    }
    onProviderChange(selection.provider);
    onModelChange(selection.model);
  }, [onModelChange, onProviderChange]);

  return (
    <div className="space-y-2.5">
      <label className={layout.labelClassName}>
        {t("agent_options.identity.model")}
      </label>
      <UiSelectMenu
        allowLabelWrap
        ariaLabel={t("agent_options.identity.model")}
        buttonClassName={layout.buttonClassName}
        className={layout.className}
        disabled={loading && options.length === 0}
        menuMinWidth={460}
        onChange={handleChange}
        options={selectOptions}
        size={layout.size}
        surface="dialog"
        value={selectedValue}
      />
      {error ? <p className={layout.errorClassName}>{error}</p> : null}
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
