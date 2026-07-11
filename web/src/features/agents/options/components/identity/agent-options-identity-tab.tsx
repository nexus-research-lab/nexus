"use client";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiTextarea } from "@/shared/ui/form/form-control";
import type { AgentNameValidationResult, AgentProvider } from "@/types/agent/agent";
import type { ProviderOption } from "@/types/capability/provider";

import {
  IDENTITY_LAYOUTS,
  type AgentIdentityVariant,
} from "./identity-layout";
import { IdentityModelSelector } from "./identity-model-selector";
import { IdentityProfileFields } from "./identity-profile-fields";
import { IdentityVibeTags } from "./identity-vibe-tags";

interface AgentOptionsIdentityTabProps {
  avatar: string;
  defaultModel: string;
  defaultProvider: AgentProvider;
  description: string;
  isValidatingName: boolean;
  model: string;
  nameValidation: AgentNameValidationResult | null;
  onAvatarChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onModelChange: (value: string) => void;
  onProviderChange: (value: AgentProvider) => void;
  onTitleChange: (value: string) => void;
  onVibeTagsChange: (tags: string[]) => void;
  provider: AgentProvider;
  providerOptions: ProviderOption[];
  providerOptionsError: string | null;
  providerOptionsLoading: boolean;
  scopeKey: string;
  title: string;
  variant?: AgentIdentityVariant;
  vibeTags: string[];
}

export function AgentOptionsIdentityTab({
  avatar,
  defaultModel,
  defaultProvider,
  description,
  isValidatingName,
  model,
  nameValidation,
  onAvatarChange,
  onDescriptionChange,
  onModelChange,
  onProviderChange,
  onTitleChange,
  onVibeTagsChange,
  provider,
  providerOptions,
  providerOptionsError,
  providerOptionsLoading,
  scopeKey,
  title,
  variant = "dialog",
  vibeTags,
}: AgentOptionsIdentityTabProps) {
  const { t } = useI18n();
  const layout = IDENTITY_LAYOUTS[variant];

  return (
    <div className="space-y-5 animate-in slide-in-from-right-4 duration-300">
      <div className={layout.contentClassName}>
        <div className={layout.profileClassName}>
          <IdentityProfileFields
            avatar={avatar}
            avatarAlt={t("agent_options.identity.avatar_alt")}
            isValidatingName={isValidatingName}
            nameAvailable={(path) => t("agent_options.identity.name_available", { path })}
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
          <IdentityModelSelector
            defaultModel={defaultModel}
            defaultProvider={defaultProvider}
            error={providerOptionsError}
            loading={providerOptionsLoading}
            model={model}
            onModelChange={onModelChange}
            onProviderChange={onProviderChange}
            options={providerOptions}
            provider={provider}
            variant={variant}
          />
        </div>
      </div>

      <div className="space-y-2">
        <label className="text-[11px] font-semibold text-(--text-muted)">
          {t("agent_options.identity.description")}
        </label>
        <UiTextarea
          className="min-h-[72px] rounded-2xl"
          onChange={(event) => onDescriptionChange(event.target.value)}
          placeholder={t("agent_options.identity.description_placeholder")}
          rows={3}
          value={description}
        />
      </div>
    </div>
  );
}
