import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderKind,
  ProviderPreset,
} from "@/types/capability/provider";

import {
  DEFAULT_AGENT_API_FORMAT,
  formatSupportsProviderKind,
  getPresetFormat,
  getSupportedPresetFormat,
  presetAllowsNonRuntimeConfig,
} from "../../model/provider-preset-model";
import type {
  FeedbackState,
  ProviderDraft,
} from "../../model/provider-settings-types";

const DEFAULT_FORMAT_BY_PROVIDER_KIND: Record<
  ProviderKind,
  ProviderApiFormat
> = {
  image_generation: "chat_completions",
  llm: DEFAULT_AGENT_API_FORMAT,
};

interface UseProviderConfigFieldsOptions {
  currentPreset: ProviderPreset | null;
  draft: ProviderDraft;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  updateDraft: (patch: Partial<ProviderDraft>) => void;
}

export function useProviderConfigFields({
  currentPreset,
  draft,
  setFeedback,
  t,
  updateDraft,
}: UseProviderConfigFieldsOptions) {
  const canSelectNonRuntimeFormat = draft.provider_kind === "llm"
    && presetAllowsNonRuntimeConfig(currentPreset);

  const handleProviderKindChange = useCallback((value: string) => {
    const providerKind = value as ProviderKind;
    const currentFormat = getPresetFormat(currentPreset, draft.api_format);
    const format = currentFormat
      && formatSupportsProviderKind(currentFormat, providerKind)
      ? currentFormat
      : getSupportedPresetFormat(currentPreset, providerKind);
    updateDraft({
      provider_kind: providerKind,
      api_format:
        format?.api_format ?? DEFAULT_FORMAT_BY_PROVIDER_KIND[providerKind],
      base_url: format?.base_url ?? draft.base_url,
      models_path: format?.models_path ?? draft.models_path,
    });
  }, [currentPreset, draft, updateDraft]);

  const handleApiFormatChange = useCallback((value: string) => {
    const apiFormat = value as ProviderApiFormat;
    const format = getPresetFormat(currentPreset, apiFormat);
    const supported = Boolean(
      format && formatSupportsProviderKind(format, draft.provider_kind),
    );
    if (!supported && !canSelectNonRuntimeFormat) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.api_format_unsupported_title"),
        message: t("settings.providers.api_format_unsupported_message"),
      });
      return;
    }
    updateDraft({
      api_format: apiFormat,
      base_url: format?.base_url ?? draft.base_url,
      models_path: format?.models_path ?? draft.models_path,
    });
  }, [
    canSelectNonRuntimeFormat,
    currentPreset,
    draft.base_url,
    draft.models_path,
    draft.provider_kind,
    setFeedback,
    t,
    updateDraft,
  ]);

  return {
    canSelectNonRuntimeFormat,
    handleApiFormatChange,
    handleProviderKindChange,
  };
}
