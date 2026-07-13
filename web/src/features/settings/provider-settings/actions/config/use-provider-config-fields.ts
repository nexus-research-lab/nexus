import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderKind,
  ProviderPreset,
} from "@/types/capability/provider";

import {
  normalizeCustomProviderKey,
} from "../../model/provider-config-model";
import {
  presetAllowsNonRuntimeConfig,
} from "../../model/provider-preset-model";
import type {
  FeedbackState,
  ProviderDraft,
} from "../../model/provider-settings-types";
import {
  buildProviderFormatTransition,
  buildProviderKindPatch,
} from "./provider-config-field-model";

interface UseProviderConfigFieldsOptions {
  currentPreset: ProviderPreset | null;
  draft: ProviderDraft;
  isCreating: boolean;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  updateDraft: (patch: Partial<ProviderDraft>) => void;
}

export function useProviderConfigFields({
  currentPreset,
  draft,
  isCreating,
  setFeedback,
  t,
  updateDraft,
}: UseProviderConfigFieldsOptions) {
  const canSelectNonRuntimeFormat = draft.provider_kind === "llm"
    && presetAllowsNonRuntimeConfig(currentPreset);

  const handleProviderDisplayNameChange = useCallback((displayName: string) => {
    updateDraft({
      display_name: displayName,
      provider: isCreating
        ? normalizeCustomProviderKey(displayName)
        : draft.provider,
    });
  }, [draft.provider, isCreating, updateDraft]);

  const handleProviderKindChange = useCallback((value: string) => {
    const providerKind = value as ProviderKind;
    updateDraft(buildProviderKindPatch(draft, currentPreset, providerKind));
  }, [currentPreset, draft, updateDraft]);

  const handleApiFormatChange = useCallback((value: string) => {
    const apiFormat = value as ProviderApiFormat;
    const transition = buildProviderFormatTransition(
      draft,
      currentPreset,
      apiFormat,
      canSelectNonRuntimeFormat,
    );
    if (transition.kind === "reject") {
      setFeedback({
        tone: "error",
        title: t("settings.providers.api_format_unsupported_title"),
        message: t("settings.providers.api_format_unsupported_message"),
      });
      return;
    }
    updateDraft(transition.patch);
  }, [
    canSelectNonRuntimeFormat,
    currentPreset,
    draft,
    setFeedback,
    t,
    updateDraft,
  ]);

  return {
    canSelectNonRuntimeFormat,
    handleApiFormatChange,
    handleProviderDisplayNameChange,
    handleProviderKindChange,
  };
}
