// INPUT: Agent 创建/编辑草稿、身份验证反馈、字段回调与模型选择插槽。
// OUTPUT: 单一字段阅读顺序、随容器换列的标签和文本/源码字段；输入交回当前草稿所有者。
// POS: Agent 身份表单组合层；共享控件持有视觉状态，保存与权限归上层流程。
"use client";

import { useId } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiField, UiTextarea } from "@/shared/ui/form/form-control";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { AgentNameValidationResult, AgentProvider } from "@/types/agent/agent";
import type { ProviderOption } from "@/types/capability/provider";

import type { AgentOptionsMode } from "../../agent-options-editor-model";
import {
  IDENTITY_CONTENT_CLASS_NAMES,
  IDENTITY_TAGS_CLASS_NAME,
  type AgentIdentityVariant,
} from "./identity-layout";
import { AgentProfileFileEditor } from "./agent-profile-file-editor";
import { IdentityModelSelector } from "./identity-model-selector";
import { IdentityProfileFields } from "./identity-profile-fields";
import { IdentityTags } from "./identity-tags";

interface AgentOptionsIdentityTabProps {
  agentId?: string;
  avatar: string;
  businessTags: string[];
  defaultModel: string;
  defaultProvider: AgentProvider;
  description: string;
  isValidatingName: boolean;
  isMain: boolean;
  model: string;
  nameValidation: AgentNameValidationResult | null;
  onAvatarChange: (value: string) => void;
  onBusinessTagsChange: (tags: string[]) => void;
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
  businessTags,
  defaultModel,
  defaultProvider,
  description,
  isValidatingName,
  isMain,
  model,
  nameValidation,
  onAvatarChange,
  onBusinessTagsChange,
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
  const descriptionId = useId();
  const templateId = useId();
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
    />
  );

  return (
    <div
      className={cn(
        "animate-in slide-in-from-right-4 duration-300 motion-reduce:animate-none",
        isInline
          ? "flex h-full min-h-0 flex-1 flex-col gap-5 overflow-hidden"
          : "space-y-6",
      )}
    >
      <div className={cn(IDENTITY_CONTENT_CLASS_NAMES[variant], isInline && "shrink-0")}>
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

        <div className={IDENTITY_TAGS_CLASS_NAME}>
          <IdentityTags
            addLabel={t("agent_options.identity.add_business_tag")}
            label={t("agent_options.identity.business_tags")}
            onChange={onBusinessTagsChange}
            resetKey={`${scopeKey}:business`}
            tags={businessTags}
          />
          <IdentityTags
            addLabel={t("agent_options.identity.add_tag")}
            label={t("agent_options.identity.vibe_tags")}
            onChange={onVibeTagsChange}
            resetKey={`${scopeKey}:vibe`}
            tags={vibeTags}
          />
        </div>

        {modelSelector}
      </div>

      {isInline && !isMain && agentId ? (
        <AgentProfileFileEditor
          agentId={agentId}
          key={agentId}
          label={t("agent_options.identity.profile_template")}
        />
      ) : null}
      {shouldShowDescriptionField ? (
        <UiField htmlFor={descriptionId} label={t("agent_options.identity.description")}>
          <UiTextarea
            id={descriptionId}
            onChange={(event) => onDescriptionChange(event.target.value)}
            placeholder={t("agent_options.identity.description_placeholder")}
            rows={3}
            value={description}
          />
        </UiField>
      ) : null}
      {sourceMode === "create" ? (
        <UiField
          description={t("agent_options.identity.profile_template_hint")}
          htmlFor={templateId}
          label={t("agent_options.identity.profile_template")}
        >
          <UiTextarea
            aria-busy={profileTemplateLoading || undefined}
            className="min-h-[180px]"
            disabled={profileTemplateLoading}
            id={templateId}
            onChange={(event) => onProfileTemplateChange(event.target.value)}
            placeholder={
              profileTemplateLoading
                ? t("agent_options.identity.profile_template_loading")
                : t("agent_options.identity.profile_template_placeholder")
            }
            rows={8}
            textRole="code"
            value={profileTemplate}
          />
          {profileTemplateError ? (
            <UiResourceState
              className="min-h-0 py-3"
              impact={t("agent_options.identity.profile_template_load_failed_impact")}
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
        </UiField>
      ) : null}
    </div>
  );
}
