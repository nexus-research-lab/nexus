import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import type { ProviderSettingsApi } from "../../provider-settings-api";
import type {
  FeedbackState,
  ProviderDraft,
} from "../../model/provider-settings-types";
import type { RunProviderCommand } from "../use-provider-command";
import { useProviderConfigFields } from "./use-provider-config-fields";
import { useProviderDelete } from "./use-provider-delete";
import { useProviderPersistence } from "./use-provider-persistence";

interface ProviderConfigContext {
  currentPreset: ProviderPreset | null;
  draft: ProviderDraft;
  isCreating: boolean;
  isEditing: boolean;
  isEmptyMode: boolean;
  providers: ProviderConfigRecord[];
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  updateDraft: (patch: Partial<ProviderDraft>) => void;
}

interface UseProviderConfigActionsOptions {
  context: ProviderConfigContext;
  providerApi: ProviderSettingsApi;
  runCommand: RunProviderCommand;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  visibilityScope: ProviderConfigRecord["visibility"];
}

export function useProviderConfigActions({
  context,
  providerApi,
  runCommand,
  setFeedback,
  t,
  visibilityScope,
}: UseProviderConfigActionsOptions) {
  const fields = useProviderConfigFields({
    currentPreset: context.currentPreset,
    draft: context.draft,
    isCreating: context.isCreating,
    setFeedback,
    t,
    updateDraft: context.updateDraft,
  });
  const persistence = useProviderPersistence({
    currentPreset: context.currentPreset,
    draft: context.draft,
    isCreating: context.isCreating,
    isEditing: context.isEditing,
    isEmptyMode: context.isEmptyMode,
    providerApi,
    refreshAll: context.refreshAll,
    runCommand,
    selectedCanManage: context.selectedCanManage,
    selectedRecord: context.selectedRecord,
    setFeedback,
    t,
    updateDraft: context.updateDraft,
    visibilityScope,
  });
  const deletion = useProviderDelete({
    providerApi,
    providers: context.providers,
    refreshAll: context.refreshAll,
    runCommand,
    setFeedback,
    t,
  });

  return { ...deletion, ...fields, ...persistence };
}
