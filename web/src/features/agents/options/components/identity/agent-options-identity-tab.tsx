"use client";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiTextarea } from "@/shared/ui/form/form-control";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { AgentNameValidationResult, AgentProvider } from "@/types/agent/agent";
import type { ProviderOption } from "@/types/capability/provider";

import type { AgentOptionsMode } from "../../agent-options-editor-model";
import {
  IDENTITY_LAYOUTS,
  type AgentIdentityVariant,
} from "./identity-layout";
import { AgentProfileFileEditor } from "./agent-profile-file-editor";
import { IdentityModelSelector } from "./identity-model-selector";
import { IdentityProfileFields } from "./identity-profile-fields";
import { IdentityVibeTags } from "./identity-vibe-tags";

interface AgentOptionsIdentityTabProps {
  agentId?: string;
  avatar: string;
  defaultModel: string;
  defaultProvider: AgentProvider;
  description: string;
  isValidatingName: boolean;
  isMain: boolean;
  model: string;
  nameValidation: AgentNameValidationResult | null;
  onAvatarChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onModelChange: (value: string) => void;
  onProfileTemplateChange: (value: string) => void;
  onRetryProfileTemplate: () => void;
  onProviderChange: (value: AgentProvider) => void;
  onTitleChange: (value: string) => void;
  onVibeTagsChange: (tags: string[]) => void;
  provider: AgentProvider;
  providerOptions: ProviderOption[];
  providerOptionsError: string | null;
  providerOptionsLoading: boolean;
  profileTemplate: string;
  profileTemplateError: string | null;
  profileTemplateLoading: boolean;
  scopeKey: string;
  sourceMode: AgentOptionsMode;
  title: string;
  variant?: AgentIdentityVariant;
  vibeTags: string[];
}

export function AgentOptionsIdentityTab({
  agentId,
  avatar,
  defaultModel,
  defaultProvider,
  description,
  isValidatingName,
  isMain,
  model,
  nameValidation,
  onAvatarChange,
  onDescriptionChange,
  onModelChange,
  onProfileTemplateChange,
  onRetryProfileTemplate,
  onProviderChange,
  onTitleChange,
  onVibeTagsChange,
  provider,
  providerOptions,
  providerOptionsError,
  providerOptionsLoading,
  profileTemplate,
  profileTemplateError,
  profileTemplateLoading,
  scopeKey,
  sourceMode,
  title,
  variant = "dialog",
  vibeTags,
}: AgentOptionsIdentityTabProps) {
  const { t } = useI18n();
  const layout = IDENTITY_LAYOUTS[variant];
  const isInline = variant === "inline";
  const shouldShowDescriptionField =
    sourceMode !== "create" && (!isInline || (!isMain && !agentId));
  const modelSelector = (
    <IdentityModelSelector
      defaultModel={defaultModel}
      defaultProvider={defaultProvider}
      error={providerOptionsError}
      lockedToDefault={isMain}
      loading={providerOptionsLoading}
      model={model}
      onModelChange={onModelChange}
      onProviderChange={onProviderChange}
      options={providerOptions}
      provider={provider}
      variant={variant}
    />
  );

  return (
    <div
      className={cn(
        "animate-in slide-in-from-right-4 duration-300",
        isInline
          ? "flex h-full min-h-0 flex-1 flex-col gap-5 overflow-hidden"
          : "space-y-6",
      )}
    >
      <div className={cn(layout.contentClassName, isInline && "shrink-0")}>
        <div className={layout.profileClassName}>
          <IdentityProfileFields
            avatar={avatar}
            avatarAlt={t("agent_options.identity.avatar_alt")}
            isValidatingName={isValidatingName}
            nameLabel={t("agent_options.identity.name")}
            namePlaceholder={t("agent_options.identity.name_placeholder")}
            nameValidation={nameValidation}
            onAvatarChange={onAvatarChange}
            onTitleChange={onTitleChange}
            title={title}
            validatingLabel={t("agent_options.identity.validating_name")}
            variant={variant}
          />
        </div>

        <div className={layout.secondaryClassName}>
          <IdentityVibeTags
            addLabel={t("agent_options.identity.add_tag")}
            label={t("agent_options.identity.vibe_tags")}
            onChange={onVibeTagsChange}
            resetKey={scopeKey}
            tags={vibeTags}
            variant={variant}
          />
        </div>

        <div className={layout.modelClassName}>
          {modelSelector}
        </div>
      </div>

      {isInline && !isMain && agentId ? (
        <AgentProfileFileEditor
          agentId={agentId}
          key={agentId}
          label={t("agent_options.identity.profile_template")}
        />
      ) : null}
      {shouldShowDescriptionField ? (
        <div className="space-y-2.5">
          <label className="text-xs font-semibold text-(--text-muted)">
            {t("agent_options.identity.description")}
          </label>
          <UiTextarea
            className="min-h-[96px] surface-radius-lg"
            onChange={(event) => onDescriptionChange(event.target.value)}
            placeholder={t("agent_options.identity.description_placeholder")}
            rows={3}
            value={description}
          />
        </div>
      ) : null}
      {sourceMode === "create" ? (
        <div className="space-y-2">
          <div>
            <label className="text-xs font-semibold text-(--text-muted)">
              {t("agent_options.identity.profile_template")}
            </label>
            <p className="mt-1 text-compact leading-5 text-(--text-soft)">
              {t("agent_options.identity.profile_template_hint")}
            </p>
          </div>
          <UiTextarea
            className="message-code-font min-h-[180px] surface-radius-lg text-sm leading-relaxed"
            disabled={profileTemplateLoading}
            onChange={(event) => onProfileTemplateChange(event.target.value)}
            placeholder={
              profileTemplateLoading
                ? t("agent_options.identity.profile_template_loading")
                : t("agent_options.identity.profile_template_placeholder")
            }
            rows={8}
            value={profileTemplate}
          />
          {profileTemplateError ? (
            <UiResourceState
              className="min-h-0 py-3"
              impact={t("agent_options.identity.profile_template_load_failed_impact")}
              nextStep={t("agent_options.identity.profile_template_load_failed_next_step")}
              primaryAction={{
                busy: profileTemplateLoading,
                label: t("state.retry"),
                onClick: onRetryProfileTemplate,
              }}
              size="sm"
              state="error"
              title={profileTemplateError}
              urgency="polite"
              variant="card"
            />
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
