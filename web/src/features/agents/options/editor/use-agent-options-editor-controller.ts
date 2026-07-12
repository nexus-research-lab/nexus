"use client";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";

import type {
  AgentOptionsControllerOptions,
  AgentOptionsEditorSource,
  AgentOptionsMode,
  AgentOptionsTabKey,
  SaveFeedback,
} from "../agent-options-editor-model";
import {
  buildAgentEditorScopeKey,
  createAgentOptionsDraft,
} from "./agent-options-draft";
import { useAgentNameValidation } from "./use-agent-name-validation";
import { useAgentOptionsDraft } from "./use-agent-options-draft";
import { useAgentOptionsSaveCommand } from "./use-agent-options-save-command";
import { useAgentProviderOptions } from "./use-agent-provider-options";
import { useAgentSaveFeedback } from "./use-agent-save-feedback";

export function useAgentOptionsEditorController({
  isActive,
  onDelete,
  onSave,
  onSaveSuccess,
  onValidateName,
  showDeleteButton = true,
  source,
  activeTab: controlledActiveTab,
  onTabChange,
}: AgentOptionsControllerOptions) {
  const { t } = useI18n();
  const sourceOptions = source.initial.options;
  const initialDraft = createAgentOptionsDraft({
    defaultTitle: t("agent_options.default_name"),
    initial: source.initial,
  });
  const scopeKey = buildAgentEditorScopeKey({
    draft: initialDraft,
    isActive,
    source,
  });
  const feedback = useAgentSaveFeedback(scopeKey);
  const draftController = useAgentOptionsDraft({
    initialDraft,
    onChange: feedback.clear,
    scopeKey,
  });
  const tabs = useAgentOptionsTabs({
    controlledActiveTab,
    onTabChange,
    scopeKey,
  });
  const providerOptions = useAgentProviderOptions(
    isActive,
    t("agent_options.identity.provider_load_failed"),
  );
  const trimmedTitle = draftController.draft.title.trim();
  const hasTitleChanged = trimmedTitle !== initialDraft.title.trim();
  const validation = useAgentNameValidation({
    fallbackError: t("agent_options.identity.validation_failed"),
    hasTitleChanged,
    isActive,
    onValidateName,
    scopeKey,
    title: draftController.draft.title,
  });
  const saveCommand = useAgentOptionsSaveCommand({
    draft: draftController.draft,
    feedback,
    hasTitleChanged,
    labels: {
      failed: t("agent_options.save_failed"),
      success: t("agent_options.save_success"),
    },
    mode: source.kind,
    onSave,
    onSaveSuccess,
    onValidateName,
    scopeKey,
    sourceOptions,
    validation,
  });
  return {
    activeTab: tabs.activeTab,
    actions: buildEditorActions({
      feedback: feedback.feedback,
      onDelete,
      saveCommand,
      showDeleteButton,
      source,
      t,
    }),
    content: {
      advanced: buildAdvancedProps(draftController),
      identity: buildIdentityProps({
        draftController,
        providerOptions,
        scopeKey,
        validation,
      }),
      skills: buildSkillsProps(source, isActive, tabs.activeTab),
    },
    onTabChange: tabs.onTabChange,
  };
}

type DraftController = ReturnType<typeof useAgentOptionsDraft>;
type SaveCommand = ReturnType<typeof useAgentOptionsSaveCommand>;
type Translate = ReturnType<typeof useI18n>["t"];

function useAgentOptionsTabs({
  controlledActiveTab,
  onTabChange,
  scopeKey,
}: {
  controlledActiveTab?: AgentOptionsTabKey;
  onTabChange?: (tab: AgentOptionsTabKey) => void;
  scopeKey: string;
}) {
  const [uncontrolledActiveTab, setUncontrolledActiveTab] =
    useResettableState<AgentOptionsTabKey>("identity", scopeKey);
  return {
    activeTab: controlledActiveTab ?? uncontrolledActiveTab,
    onTabChange: onTabChange ?? setUncontrolledActiveTab,
  };
}

function buildEditorActions({
  feedback,
  onDelete,
  saveCommand,
  showDeleteButton,
  source,
  t,
}: {
  feedback: SaveFeedback | null;
  onDelete?: (agentId: string) => void;
  saveCommand: SaveCommand;
  showDeleteButton: boolean;
  source: AgentOptionsEditorSource;
  t: Translate;
}) {
  return {
    deleteAction: buildDeleteAction({
      label: t("agent_options.delete_agent"),
      onDelete,
      showDeleteButton,
      source,
    }),
    feedback,
    saveAction: {
      enabled: saveCommand.canSave,
      label: resolveSaveButtonLabel({
        feedback,
        isSaving: saveCommand.isSaving,
        mode: source.kind,
        labels: {
          create: t("agent_options.title_create"),
          error: t("agent_options.save_failed"),
          save: t("agent_options.save_changes"),
          saving: t("common.saving"),
          success: t("agent_options.save_success"),
        },
      }),
      run: saveCommand.save,
    },
  };
}

function buildDeleteAction({
  label,
  onDelete,
  showDeleteButton,
  source,
}: {
  label: string;
  onDelete?: (agentId: string) => void;
  showDeleteButton: boolean;
  source: AgentOptionsEditorSource;
}) {
  if (!showDeleteButton || source.kind !== "edit" || !onDelete) {
    return null;
  }
  return {
    label,
    run: () => onDelete(source.agentId),
  };
}

function buildSkillsProps(
  source: AgentOptionsEditorSource,
  isActive: boolean,
  activeTab: AgentOptionsTabKey,
) {
  return {
    agentId: source.kind === "edit" ? source.agentId : undefined,
    isVisible: [isActive, activeTab === "skills"].every(Boolean),
  };
}

function buildAdvancedProps({
  draft,
  toggleTool,
  updateField,
}: DraftController) {
  return {
    allowedTools: draft.allowedTools,
    onPermissionModeChange: (value: string) => updateField("permissionMode", value),
    onToggleTool: toggleTool,
    permissionMode: draft.permissionMode,
  };
}

function buildIdentityProps({
  draftController: { draft, updateField },
  providerOptions,
  scopeKey,
  validation,
}: {
  draftController: DraftController;
  providerOptions: ReturnType<typeof useAgentProviderOptions>;
  scopeKey: string;
  validation: ReturnType<typeof useAgentNameValidation>;
}) {
  return {
    avatar: draft.avatar,
    defaultModel: providerOptions.defaultModel,
    defaultProvider: providerOptions.defaultProvider,
    description: draft.description,
    isValidatingName: validation.isValidating,
    model: draft.model,
    nameValidation: validation.result,
    onAvatarChange: (value: string) => updateField("avatar", value),
    onDescriptionChange: (value: string) => updateField("description", value),
    onModelChange: (value: string) => updateField("model", value),
    onProviderChange: (value: string) => updateField("provider", value),
    onTitleChange: (value: string) => updateField("title", value),
    onVibeTagsChange: (value: string[]) => updateField("vibeTags", value),
    provider: draft.provider,
    providerOptions: providerOptions.items,
    providerOptionsError: providerOptions.error,
    providerOptionsLoading: providerOptions.loading,
    scopeKey,
    title: draft.title,
    vibeTags: draft.vibeTags,
  };
}

function resolveSaveButtonLabel({
  feedback,
  isSaving,
  labels,
  mode,
}: {
  feedback: SaveFeedback | null;
  isSaving: boolean;
  labels: Record<"create" | "error" | "save" | "saving" | "success", string>;
  mode: AgentOptionsMode;
}): string {
  const candidates = [
    { active: isSaving, label: labels.saving },
    { active: feedback?.tone === "success", label: labels.success },
    { active: feedback?.tone === "error", label: labels.error },
    { active: mode === "create", label: labels.create },
  ];
  return candidates.find((candidate) => candidate.active)?.label ?? labels.save;
}
