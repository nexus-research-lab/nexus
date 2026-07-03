import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { Locale } from "@/shared/i18n/messages";
import type { Theme } from "@/shared/theme/theme-context";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

export const DELIVERY_POLICY_OPTIONS: ReadonlyArray<{
  value: AgentConversationDefaultDeliveryPolicy;
  labelKey: "settings.general.default_delivery_queue" | "settings.general.default_delivery_interrupt";
}> = [
  { value: "queue", labelKey: "settings.general.default_delivery_queue" },
  { value: "interrupt", labelKey: "settings.general.default_delivery_interrupt" },
];

export const AGENT_RUNTIME_KIND_OPTIONS: ReadonlyArray<{
  value: AgentRuntimeKind;
  labelKey: "settings.general.runtime_claude" | "settings.general.runtime_nxs";
}> = [
  { value: "claude", labelKey: "settings.general.runtime_claude" },
  { value: "nxs", labelKey: "settings.general.runtime_nxs" },
];

export const THEME_OPTIONS: ReadonlyArray<{
  value: Theme;
  labelKey: "theme.light" | "theme.dark" | "theme.sunny" | "theme.rain";
}> = [
  { value: "light", labelKey: "theme.light" },
  { value: "dark", labelKey: "theme.dark" },
  { value: "sunny", labelKey: "theme.sunny" },
  { value: "rain", labelKey: "theme.rain" },
];

export const LOCALE_OPTIONS: ReadonlyArray<{
  value: Locale;
  labelKey: "language.zh" | "language.en";
}> = [
  { value: "zh", labelKey: "language.zh" },
  { value: "en", labelKey: "language.en" },
];
