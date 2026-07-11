"use client";

import { useCallback } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";

import type {
  AgentEditorInitialOptions,
  AgentOptionsControllerOptions,
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
  agentId,
  mode,
  isActive,
  onDelete,
  onSave,
  onSaveSuccess,
  onValidateName,
  initialTitle = "",
  initialOptions = {},
  initialAvatar = "",
  initialDescription = "",
  initialVibeTags = [],
  showDeleteButton = true,
  activeTab: controlledActiveTab,
  onTabChange,
}: AgentOptionsControllerOptions) {
  const { t } = useI18n();
  const sourceOptions = initialOptions as AgentEditorInitialOptions;
  const initialDraft = createAgentOptionsDraft({
    defaultTitle: t("agent_options.default_name"),
    initialAvatar,
    initialDescription,
    initialOptions: sourceOptions,
    initialTitle,
    initialVibeTags,
  });
  const scopeKey = buildAgentEditorScopeKey({
    draft: initialDraft,
    initialOptions: sourceOptions,
    props: { agentId, isActive, mode },
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
    mode,
    onSave,
    onSaveSuccess,
    onValidateName,
    scopeKey,
    sourceOptions,
    validation,
  });
  const handleDelete = useCallback(() => {
    onDelete?.(agentId ?? "");
  }, [agentId, onDelete]);

  return {
    activeTab: tabs.activeTab,
    actions: buildEditorActions({
      agentId,
      feedback: feedback.feedback,
      handleDelete,
      mode,
      onDelete,
      saveCommand,
      showDeleteButton,
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
      skills: buildSkillsProps(agentId, isActive, mode, tabs.activeTab),
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
  agentId,
  feedback,
  handleDelete,
  mode,
  onDelete,
  saveCommand,
  showDeleteButton,
  t,
}: {
  agentId?: string;
  feedback: SaveFeedback | null;
  handleDelete: () => void;
  mode: AgentOptionsMode;
  onDelete?: (agentId: string) => void;
  saveCommand: SaveCommand;
  showDeleteButton: boolean;
  t: Translate;
}) {
  return {
    deleteAction: buildDeleteAction({
      agentId,
      handleDelete,
      label: t("agent_options.delete_agent"),
      mode,
      onDelete,
      showDeleteButton,
    }),
    feedback,
    saveAction: {
      enabled: saveCommand.canSave,
      label: resolveSaveButtonLabel({
        feedback,
        isSaving: saveCommand.isSaving,
        mode,
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
  agentId,
  handleDelete,
  label,
  mode,
  onDelete,
  showDeleteButton,
}: {
  agentId?: string;
  handleDelete: () => void;
  label: string;
  mode: AgentOptionsMode;
  onDelete?: (agentId: string) => void;
  showDeleteButton: boolean;
}) {
  const rules = [
    {
      matches: [
        showDeleteButton,
        mode === "edit",
        Boolean(agentId),
        Boolean(onDelete),
      ].every(Boolean),
      value: { label, run: handleDelete },
    },
    { matches: true, value: null },
  ];
  return rules.find((rule) => rule.matches)!.value;
}

function buildSkillsProps(
  agentId: string | undefined,
  isActive: boolean,
  mode: AgentOptionsMode,
  activeTab: AgentOptionsTabKey,
) {
  const agentIdByMode: Readonly<Record<AgentOptionsMode, string | undefined>> = {
    create: undefined,
    edit: agentId,
  };
  return {
    agentId: agentIdByMode[mode],
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
