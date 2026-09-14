// INPUT: Gallery locale and isolated Agent option fixture state.
// OUTPUT: Real permission and Skill views for responsive, switch-only and disabled behavior checks.
// POS: Development-only product fixture; all commands update local state and never call product services.

import { useState } from "react";
import { MemoryRouter } from "react-router-dom";
import { AgentOptionsAdvancedTab } from "@/features/agents/options/components/agent-options-advanced-tab";
import { AgentSkillCard } from "@/features/agents/options/components/skills/agent-skill-card";
import { AgentOptionsIdentityTab } from "@/features/agents/options/components/identity/agent-options-identity-tab";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { Locale } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ConnectorInfo } from "@/types/capability/connector";
import type { AgentSkillEntry } from "@/types/capability/skill";
import { galleryText } from "./ui-gallery-copy";

const CONNECTORS: ConnectorInfo[] = ["Available connector", "Unavailable connector", "Previously enabled connector"].map((title, index) => ({
  connector_id: title, name: title, title, connection_state: index === 0 ? "connected" : "disconnected",
  auth_type: "oauth2", category: "productivity", description: "Read project documents and compare the available capabilities.",
  icon: "github", is_configured: true, kind: "connector", status: "available",
}));

const SKILL: AgentSkillEntry = {
  name: "gallery-review", title: "Review sample", description: "Check proposed changes and preserve the existing product behavior.",
  scope: "any", tags: [], category_key: "development", category_name: "Development", source_type: "external",
  source_ref: "", version: "1", enabled_for_agent: false, locked: false, has_update: false, deletable: true,
};

function toggleValue(values: string[], value: string) {
  return values.includes(value) ? values.filter((item) => item !== value) : [...values, value];
}

export function AgentOptionsGallery({ locale }: { locale: Locale }) {
  const [permissionMode, setPermissionMode] = useState("default");
  const [connectorIds, setConnectorIds] = useState(["Previously enabled connector"]);
  const [skillEnabled, setSkillEnabled] = useState(false);
  const [skillBusy, setSkillBusy] = useState(false);
  return (
    <section className="min-w-0 space-y-5" data-gallery-agent-options>
      <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
        {galleryText(locale, "Agent 配置真实视图", "Agent configuration views")}
      </h2>
      <div className="grid gap-5 lg:grid-cols-2" data-gallery-identity-fields>
        <AgentIdentityFixture locale={locale} variant="dialog" />
        <AgentIdentityFixture locale={locale} variant="inline" />
      </div>
      <div className="grid gap-3 sm:grid-cols-2" data-gallery-wrapping-selects>
        {(["xs", "sm", "md", "lg"] as const).map((size) => (
          <UiSelectMenu allowLabelWrap ariaLabel={`Wrapping ${size}`} key={size} onChange={() => undefined}
            options={[{ value: "long", label: "ProviderWithAnUnusuallyLongName / ReasoningModelWithExtendedContextAndRegionalRouting" }]}
            size={size} value="long" />
        ))}
      </div>
      <div data-gallery-agent-permissions>
        <MemoryRouter>
          <AgentOptionsAdvancedTab
            connectorIds={connectorIds} connectors={CONNECTORS}
            connectorsError={null} connectorsLoading={false} onPermissionModeChange={setPermissionMode}
            onRetryConnectors={() => undefined} onToggleConnector={(id) => setConnectorIds((values) => toggleValue(values, id))}
            permissionMode={permissionMode}
          />
        </MemoryRouter>
      </div>
      <div className="grid gap-3 sm:grid-cols-2" data-gallery-agent-skills>
        <AgentSkillCard actionLabel="Toggle" blocked={false} busy={skillBusy} commandBusy={skillBusy}
          onAction={() => setSkillEnabled((current) => !current)} skill={{ ...SKILL, enabled_for_agent: skillEnabled }} />
        <AgentSkillCard actionLabel="Toggle" blocked={false} busy={false} commandBusy={false}
          onAction={() => undefined} skill={{ ...SKILL, name: "gallery-core-review", title: "Core review", locked: true, source_type: "system" }} />
      </div>
      <UiButton aria-pressed={skillBusy} onClick={() => setSkillBusy((current) => !current)}>
        Toggle pending Skill
      </UiButton>
    </section>
  );
}

function AgentIdentityFixture({ locale, variant }: { locale: Locale; variant: "dialog" | "inline" }) {
  const [title, setTitle] = useState("Nova · Research and planning");
  const [avatar, setAvatar] = useState("1");
  const [businessTags, setBusinessTags] = useState(["Research", "Product planning", "跨产品协作与长期研究"]);
  const [vibeTags, setVibeTags] = useState(["Concise", "Thoughtful"]);
  const [description, setDescription] = useState("");
  const [profileTemplate, setProfileTemplate] = useState("# Working rules\nKeep the scope clear.");
  const [model, setModel] = useState("");
  const [provider, setProvider] = useState("");
  const [nameError, setNameError] = useState(false);
  const [templateLoading, setTemplateLoading] = useState(false);
  return (
    <div className="min-w-0 space-y-3" data-gallery-identity-variant={variant}>
      <AgentOptionsIdentityTab
        avatar={avatar} businessTags={businessTags}
        defaultModel="Reasoning model with extended context and regional routing" defaultProvider="Provider with an unusually long name"
        description={description} isMain={variant === "inline"} isValidatingName={false} model={model}
        nameValidation={nameError ? { name: title, normalized_name: title, is_valid: false, is_available: true,
          reason: galleryText(locale, "名称不能包含换行，请修改后继续。当前输入不会丢失。", "The name cannot contain a newline. Edit it to continue; your current input is preserved.") } : null}
        onAvatarChange={setAvatar} onBusinessTagsChange={setBusinessTags} onDescriptionChange={setDescription}
        onModelChange={setModel} onProfileTemplateChange={setProfileTemplate} onProviderChange={setProvider}
        onRetryProfileTemplate={() => undefined} onTitleChange={setTitle} onVibeTagsChange={setVibeTags}
        profileTemplate={profileTemplate} profileTemplateError={null} profileTemplateLoading={templateLoading}
        provider={provider} providerOptions={[]} providerOptionsError={null} providerOptionsLoading={false}
        scopeKey={`gallery-${variant}`} sourceMode={variant === "dialog" ? "create" : "edit"} title={title}
        variant={variant} vibeTags={vibeTags}
      />
      <div className="flex flex-wrap gap-2">
        <UiButton aria-pressed={nameError} onClick={() => setNameError((current) => !current)}>Toggle name error</UiButton>
        {variant === "dialog" ? <UiButton aria-pressed={templateLoading} onClick={() => setTemplateLoading((current) => !current)}>Toggle template loading</UiButton> : null}
      </div>
    </div>
  );
}
