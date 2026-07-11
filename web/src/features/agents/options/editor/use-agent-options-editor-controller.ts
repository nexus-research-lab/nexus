"use client";

import { useCallback, useRef, useState } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { AgentNameValidationResult } from "@/types/agent/agent";

import type { TabKey } from "../components/agent-options-nav";
import type {
  AgentDialogInitialOptions,
  AgentOptionsEditorProps,
  SaveFeedback,
} from "../agent-options-editor-model";
import {
  buildAgentEditorScopeKey,
  buildAgentOptionsSubmission,
  createAgentOptionsDraft,
} from "./agent-options-draft";
import { useAgentNameValidation } from "./use-agent-name-validation";
import { useAgentOptionsDraft } from "./use-agent-options-draft";
import { useAgentProviderOptions } from "./use-agent-provider-options";
import { useAgentSaveFeedback } from "./use-agent-save-feedback";

interface SaveToken {
  draftKey: string;
  id: number;
  scopeKey: string;
}

export function useAgentOptionsEditorController({
  agentId,
  mode,
  isActive,
  onDelete,
  onSave,
  onValidateName,
  initialTitle = "",
  initialOptions = {},
  initialAvatar = "",
  initialDescription = "",
  initialVibeTags = [],
  onCancel,
  closeAfterSave = false,
  showCancelButton = true,
  showDeleteButton = true,
  variant = "dialog",
  contentMaxWidthClassName = "max-w-[920px]",
  activeTab: controlledActiveTab,
  onTabChange,
  hideInlineNav = false,
}: AgentOptionsEditorProps) {
  const { t } = useI18n();
  const sourceOptions = initialOptions as AgentDialogInitialOptions;
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
  const scopeKeyRef = useRef(scopeKey);
  scopeKeyRef.current = scopeKey;

  const feedback = useAgentSaveFeedback(scopeKey);
  const { draft, toggleTool, updateField } = useAgentOptionsDraft({
    initialDraft,
    onChange: feedback.clear,
    scopeKey,
  });
  const draftKey = JSON.stringify(draft);
  const draftKeyRef = useRef(draftKey);
  draftKeyRef.current = draftKey;
  const [uncontrolledActiveTab, setUncontrolledActiveTab] =
    useResettableState<TabKey>("identity", scopeKey);
  const activeTab = controlledActiveTab ?? uncontrolledActiveTab;
  const setActiveTab = onTabChange ?? setUncontrolledActiveTab;
  const providerOptions = useAgentProviderOptions(
    isActive,
    t("agent_options.identity.provider_load_failed"),
  );
  const trimmedTitle = draft.title.trim();
  const hasTitleChanged = trimmedTitle !== initialDraft.title.trim();
  const validation = useAgentNameValidation({
    fallbackError: t("agent_options.identity.validation_failed"),
    hasTitleChanged,
    isActive,
    onValidateName,
    scopeKey,
    title: draft.title,
  });

  const saveSequenceRef = useRef(0);
  const saveTokenRef = useRef<SaveToken | null>(null);
  const [savingScopeKey, setSavingScopeKey] = useState<string | null>(null);
  const isSaving = savingScopeKey === scopeKey;
  const nameIsInvalid = isInvalidNameValidation(validation.result);

  const handleSave = useCallback(async () => {
    if (
      !trimmedTitle
      || validation.isValidating
      || saveTokenRef.current?.scopeKey === scopeKey
    ) {
      return;
    }
    const token = {
      draftKey,
      id: saveSequenceRef.current + 1,
      scopeKey,
    };
    saveSequenceRef.current = token.id;
    saveTokenRef.current = token;
    setSavingScopeKey(scopeKey);
    feedback.clear();

    try {
      const requiresValidation = Boolean(onValidateName)
        && (mode === "create" || hasTitleChanged);
      let result = validation.result;
      if (requiresValidation && result?.name !== trimmedTitle) {
        result = await validation.validateNow(trimmedTitle);
      }
      if (!isCurrentSave(token, saveTokenRef.current, scopeKeyRef, draftKeyRef)) {
        return;
      }
      if (requiresValidation && isInvalidNameValidation(result)) {
        return;
      }

      const submission = buildAgentOptionsSubmission(draft, sourceOptions);
      await onSave(submission.title, submission.options, submission.identity);
      if (!isCurrentSave(token, saveTokenRef.current, scopeKeyRef, draftKeyRef)) {
        return;
      }
      if (closeAfterSave) {
        onCancel?.();
      } else {
        feedback.showSuccess(t("agent_options.save_success"));
      }
    } catch (error) {
      if (isCurrentSave(token, saveTokenRef.current, scopeKeyRef, draftKeyRef)) {
        feedback.showError(
          error instanceof Error ? error.message : t("agent_options.save_failed"),
        );
      }
    } finally {
      if (saveTokenRef.current?.id === token.id) {
        saveTokenRef.current = null;
        setSavingScopeKey(null);
      }
    }
  }, [
    closeAfterSave,
    draft,
    draftKey,
    feedback,
    hasTitleChanged,
    mode,
    onCancel,
    onSave,
    onValidateName,
    scopeKey,
    sourceOptions,
    t,
    trimmedTitle,
    validation,
  ]);

  const handleDelete = useCallback(() => {
    if (agentId && onDelete) {
      onDelete(agentId);
    }
  }, [agentId, onDelete]);
  const canSave = Boolean(trimmedTitle)
    && !validation.isValidating
    && !nameIsInvalid
    && !isSaving;
  const canDelete = showDeleteButton
    && mode === "edit"
    && Boolean(agentId)
    && Boolean(onDelete);

  return {
    activeTab,
    setActiveTab,
    advancedProps: {
      permissionMode: draft.permissionMode,
      onPermissionModeChange: (value: string) => updateField("permissionMode", value),
      allowedTools: draft.allowedTools,
      onToggleTool: toggleTool,
    },
    canDelete,
    canSave,
    cancelLabel: t("common.cancel"),
    contentMaxWidthClassName,
    deleteAgentLabel: t("agent_options.delete_agent"),
    handleDelete,
    handleSave,
    hideInlineNav,
    identityProps: {
      avatar: draft.avatar,
      onAvatarChange: (value: string) => updateField("avatar", value),
      title: draft.title,
      onTitleChange: (value: string) => updateField("title", value),
      description: draft.description,
      onDescriptionChange: (value: string) => updateField("description", value),
      vibeTags: draft.vibeTags,
      onVibeTagsChange: (value: string[]) => updateField("vibeTags", value),
      provider: draft.provider,
      model: draft.model,
      defaultProvider: providerOptions.defaultProvider,
      defaultModel: providerOptions.defaultModel,
      providerOptions: providerOptions.items,
      providerOptionsError: providerOptions.error,
      providerOptionsLoading: providerOptions.loading,
      onProviderChange: (value: string) => updateField("provider", value),
      onModelChange: (value: string) => updateField("model", value),
      nameValidation: validation.result,
      isValidatingName: validation.isValidating,
      variant,
    },
    isActive,
    mode,
    onCancel,
    saveButtonLabel: resolveSaveButtonLabel({
      feedback: feedback.feedback,
      isSaving,
      mode,
      labels: {
        create: t("agent_options.title_create"),
        error: t("agent_options.save_failed"),
        save: t("agent_options.save_changes"),
        saving: t("common.saving"),
        success: t("agent_options.save_success"),
      },
    }),
    saveFeedback: feedback.feedback,
    showCancelButton,
    skillsAgentId: mode === "edit" ? agentId : undefined,
    variant,
  };
}

function isInvalidNameValidation(
  result: AgentNameValidationResult | null,
): boolean {
  return Boolean(result && (!result.is_valid || !result.is_available));
}

function isCurrentSave(
  expected: SaveToken,
  current: SaveToken | null,
  currentScopeKey: { current: string },
  currentDraftKey: { current: string },
): boolean {
  return current?.id === expected.id
    && currentScopeKey.current === expected.scopeKey
    && currentDraftKey.current === expected.draftKey;
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
  mode: AgentOptionsEditorProps["mode"];
}): string {
  const candidates = [
    { active: isSaving, label: labels.saving },
    { active: feedback?.tone === "success", label: labels.success },
    { active: feedback?.tone === "error", label: labels.error },
    { active: mode === "create", label: labels.create },
  ];
  return candidates.find((candidate) => candidate.active)?.label ?? labels.save;
}
