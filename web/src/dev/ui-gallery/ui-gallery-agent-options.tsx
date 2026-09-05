// INPUT: Gallery locale and isolated Agent option fixture state.
// OUTPUT: Real permission and Skill views for responsive, switch-only and disabled behavior checks.
// POS: Development-only product fixture; all commands update local state and never call product services.

import { useState } from "react";
import { AgentOptionsAdvancedTab } from "@/features/agents/options/components/agent-options-advanced-tab";
import { AgentSkillCard } from "@/features/agents/options/components/skills/agent-skill-card";
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
  const [allowedTools, setAllowedTools] = useState(["Bash"]);
  const [connectorIds, setConnectorIds] = useState(["Previously enabled connector"]);
  const [skillEnabled, setSkillEnabled] = useState(false);
  const [skillBusy, setSkillBusy] = useState(false);
  return (
    <section className="min-w-0 space-y-5" data-gallery-agent-options>
      <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
        {galleryText(locale, "Agent 配置真实视图", "Agent configuration views")}
      </h2>
      <div data-gallery-agent-permissions>
        <AgentOptionsAdvancedTab
          allowedTools={allowedTools} connectorIds={connectorIds} connectors={CONNECTORS}
          connectorsError={null} connectorsLoading={false} onPermissionModeChange={setPermissionMode}
          onRetryConnectors={() => undefined} onToggleConnector={(id) => setConnectorIds((values) => toggleValue(values, id))}
          onToggleTool={(name) => setAllowedTools((values) => toggleValue(values, name))} permissionMode={permissionMode}
        />
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
