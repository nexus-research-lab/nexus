"use client";

import { useEffect, useMemo, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";

import type {
  AgentOptionsControllerOptions,
  AgentOptionsEditorSource,
  AgentOptionsMode,
  AgentOptionsPersistenceState,
  AgentOptionsTabKey,
  SaveFeedback,
} from "../agent-options-editor-model";
import {
  buildAgentEditorCommandScopeKey,
  buildAgentEditorScopeKey,
  createAgentOptionsDraft,
} from "./agent-options-draft";
import { useAgentNameValidation } from "./use-agent-name-validation";
import { useAgentConnectors } from "./use-agent-connectors";
import { useAgentOptionsAutoSave } from "./use-agent-options-auto-save";
import { useAgentOptionsDraft } from "./use-agent-options-draft";
import { useAgentProfileTemplate } from "./use-agent-profile-template";
import { useAgentOptionsSaveCommand } from "./use-agent-options-save-command";
import { useAgentProviderOptions } from "./use-agent-provider-options";
import { useAgentSaveFeedback } from "./use-agent-save-feedback";

export function useAgentOptionsEditorController({
  isActive,
  onDelete,
  onSave,
  onSaveSuccess,
  onValidateName,
  saveMode = "explicit",
  showDeleteButton = true,
  source,
  activeTab: controlledActiveTab,
  onTabChange,
}: AgentOptionsControllerOptions) {
  const { t } = useI18n();
  const sourceOptions = source.initial.options;
  const initialDraft = useMemo(() => createAgentOptionsDraft({
    defaultTitle: t("agent_options.default_name"),
    initial: source.initial,
  }), [source.initial, t]);
  const sourceScopeKey = buildAgentEditorScopeKey({
    draft: initialDraft,
    isActive,
    source,
  });
  const commandScopeKey = buildAgentEditorCommandScopeKey({ isActive, source });
  const feedback = useAgentSaveFeedback(commandScopeKey);
  const draftController = useAgentOptionsDraft({
    editorScopeKey: commandScopeKey,
    initialDraft,
    onChange: feedback.clearForEdit,
    sourceScopeKey,
  });
  const profileTemplate = useAgentProfileTemplate(
    source.kind === "create" && isActive,
    sourceScopeKey,
    t("agent_options.identity.profile_template_load_failed"),
  );
  const updateDraftField = draftController.updateField;
  const appliedProfileTemplateScopeRef = useRef<string | null>(null);
  useEffect(() => {
    if (
      source.kind !== "create"
      || !profileTemplate.content
      || appliedProfileTemplateScopeRef.current === sourceScopeKey
    ) {
      return;
    }
    appliedProfileTemplateScopeRef.current = sourceScopeKey;
    updateDraftField("profileTemplate", profileTemplate.content);
  }, [
    profileTemplate.content,
    sourceScopeKey,
    source.kind,
    updateDraftField,
  ]);
  const tabs = useAgentOptionsTabs({
    controlledActiveTab,
    onTabChange,
    scopeKey: commandScopeKey,
  });
  const connectors = useAgentConnectors(
    isActive && tabs.activeTab === "advanced",
    t("agent_options.advanced.connector_load_failed"),
  );
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
    scopeKey: sourceScopeKey,
    title: draftController.draft.title,
  });
  const saveCommand = useAgentOptionsSaveCommand({
    commandScopeKey,
    draft: draftController.draft,
    draftRevisionRef: draftController.revisionRef,
    feedback,
    hasTitleChanged,
    labels: {
      failed: t(source.kind === "create"
        ? "agent_options.create_failed"
        : "agent_options.save_failed"),
      failures: {
        accepted: {
          impact: t(source.kind === "create" ? "agent_options.create_accepted_impact" : "agent_options.save_accepted_impact"),
          title: t(source.kind === "create" ? "agent_options.create_accepted_message" : "agent_options.save_accepted_message"),
        },
        committed: {
          impact: t(source.kind === "create" ? "agent_options.create_committed_impact" : "agent_options.save_committed_impact"),
          title: t(source.kind === "create" ? "agent_options.create_committed_message" : "agent_options.save_committed_message"),
        },
        not_applied: {
          impact: t(source.kind === "create" ? "agent_options.create_not_applied_impact" : "agent_options.save_not_applied_impact"),
          title: t(source.kind === "create" ? "agent_options.create_not_applied_message" : "agent_options.save_not_applied_message"),
        },
        unknown: {
          impact: t(source.kind === "create" ? "agent_options.create_unknown_impact" : "agent_options.save_unknown_impact"),
          title: t(source.kind === "create" ? "agent_options.create_unknown_message" : "agent_options.save_unknown_message"),
        },
      },
      success: t(
        source.kind === "create"
          ? "agent_options.create_success"
          : saveMode === "automatic"
          ? "agent_options.auto_save_success"
          : "agent_options.save_success",
      ),
    },
    mode: source.kind,
    onSave,
    onSaveSuccess,
    onValidateName,
    sourceScopeKey,
    sourceOptions,
    validation,
  });
  const saveEnabled = saveCommand.canSave && !profileTemplate.loading;
  const automaticSaveEnabled = saveMode === "automatic" && source.kind === "edit";
  useAgentOptionsAutoSave({
    canSave: saveEnabled,
    draftRevision: draftController.revision,
    enabled: automaticSaveEnabled,
    isDirty: draftController.isDirty,
    isSaving: saveCommand.isSaving,
    save: saveCommand.save,
    scopeKey: commandScopeKey,
  });
  return {
    activeTab: tabs.activeTab,
    actions: buildEditorActions({
      feedback: feedback.feedback,
      profileTemplateLoading: profileTemplate.loading,
      onDelete,
      saveCommand,
      showDeleteButton,
      source,
      t,
    }),
    content: {
      advanced: buildAdvancedProps(draftController, connectors),
      identity: buildIdentityProps({
        draftController,
        profileTemplate,
        providerOptions,
        scopeKey: sourceScopeKey,
        source,
        validation,
      }),
      skills: buildSkillsProps(source, isActive, tabs.activeTab),
    },
    onTabChange: tabs.onTabChange,
    persistence: buildPersistenceState({
      automaticSaveEnabled,
      feedback: feedback.feedback,
      isDirty: draftController.isDirty,
      isSaving: saveCommand.isSaving,
      t,
    }),
  };
}

type DraftController = ReturnType<typeof useAgentOptionsDraft>;
type SaveCommand = ReturnType<typeof useAgentOptionsSaveCommand>;
type Translate = ReturnType<typeof useI18n>["t"];

function buildPersistenceState({
  automaticSaveEnabled,
  feedback,
  isDirty,
  isSaving,
  t,
}: {
  automaticSaveEnabled: boolean;
  feedback: SaveFeedback | null;
  isDirty: boolean;
  isSaving: boolean;
  t: Translate;
}): AgentOptionsPersistenceState {
  if (!automaticSaveEnabled) {
    return { message: "", phase: "idle" };
  }
  if (isSaving) {
    return { message: t("common.saving"), phase: "saving" };
  }
  if (feedback?.tone === "error" || feedback?.tone === "warning") {
    return {
      message: [feedback.title, feedback.impact].join(" "),
      phase: "error",
    };
  }
  if (feedback?.tone === "success") {
    return { message: feedback.message, phase: "success" };
  }
  return {
    message: t("agent_options.auto_save"),
    phase: isDirty ? "pending" : "idle",
  };
}

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
  profileTemplateLoading,
  saveCommand,
  showDeleteButton,
  source,
  t,
}: {
  feedback: SaveFeedback | null;
  onDelete?: (agentId: string) => void;
  profileTemplateLoading: boolean;
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
      enabled: saveCommand.canSave && !profileTemplateLoading,
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

function buildAdvancedProps(
  { draft, toggleTool, updateField }: DraftController,
  connectors: ReturnType<typeof useAgentConnectors>,
) {
  return {
    allowedTools: draft.allowedTools,
    connectorIds: draft.connectorIds,
    connectors: connectors.items,
    connectorsError: connectors.error,
    connectorsLoading: connectors.loading,
    onRetryConnectors: connectors.retry,
    onPermissionModeChange: (value: string) => updateField("permissionMode", value),
    onToggleConnector: (connectorId: string) => updateField(
      "connectorIds",
      draft.connectorIds.includes(connectorId)
        ? draft.connectorIds.filter((value) => value !== connectorId)
        : [...draft.connectorIds, connectorId],
    ),
    onToggleTool: toggleTool,
    permissionMode: draft.permissionMode,
  };
}

function buildIdentityProps({
  draftController: { draft, updateField },
  profileTemplate,
  providerOptions,
  scopeKey,
  source,
  validation,
}: {
  draftController: DraftController;
  profileTemplate: ReturnType<typeof useAgentProfileTemplate>;
  providerOptions: ReturnType<typeof useAgentProviderOptions>;
  scopeKey: string;
  source: AgentOptionsEditorSource;
  validation: ReturnType<typeof useAgentNameValidation>;
}) {
  const isMain = source.kind === "edit" && source.isMain;
  return {
    agentId: source.kind === "edit" ? source.agentId : undefined,
    avatar: draft.avatar,
    businessTags: draft.businessTags,
    defaultModel: providerOptions.defaultModel,
    defaultProvider: providerOptions.defaultProvider,
    description: draft.description,
    isValidatingName: validation.isValidating,
    isMain,
    model: isMain ? "" : draft.model,
    nameValidation: validation.result,
    onAvatarChange: (value: string) => updateField("avatar", value),
    onBusinessTagsChange: (value: string[]) => updateField("businessTags", value),
    onDescriptionChange: (value: string) => updateField("description", value),
    onProfileTemplateChange: (value: string) => updateField("profileTemplate", value),
    onModelChange: (value: string) => updateField("model", value),
    onProviderChange: (value: string) => updateField("provider", value),
    onTitleChange: (value: string) => updateField("title", value),
    onVibeTagsChange: (value: string[]) => updateField("vibeTags", value),
    provider: isMain ? "" : draft.provider,
    providerOptions: providerOptions.items,
    providerOptionsError: providerOptions.error,
    providerOptionsLoading: providerOptions.loading,
    profileTemplate: draft.profileTemplate ?? "",
    profileTemplateError: profileTemplate.error,
    profileTemplateLoading: profileTemplate.loading,
    onRetryProfileTemplate: profileTemplate.retry,
    scopeKey,
    sourceMode: source.kind,
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
    {
      active: feedback?.tone === "error" || feedback?.tone === "warning",
      label: labels.error,
    },
    { active: mode === "create", label: labels.create },
  ];
  return candidates.find((candidate) => candidate.active)?.label ?? labels.save;
}
