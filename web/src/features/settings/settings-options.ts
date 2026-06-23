import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { Locale } from "@/shared/i18n/messages";
import type { Theme } from "@/shared/theme/theme-context";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

export const DELIVERY_POLICY_OPTIONS: ReadonlyArray<{
  value: AgentConversationDefaultDeliveryPolicy;
  label_key: "settings.general.default_delivery_queue" | "settings.general.default_delivery_interrupt";
}> = [
  { value: "queue", label_key: "settings.general.default_delivery_queue" },
  { value: "interrupt", label_key: "settings.general.default_delivery_interrupt" },
];

export const AGENT_RUNTIME_KIND_OPTIONS: ReadonlyArray<{
  value: AgentRuntimeKind;
  label_key: "settings.general.runtime_claude" | "settings.general.runtime_nxs";
}> = [
  { value: "claude", label_key: "settings.general.runtime_claude" },
  { value: "nxs", label_key: "settings.general.runtime_nxs" },
];

export const THEME_OPTIONS: ReadonlyArray<{
  value: Theme;
  label_key: "theme.light" | "theme.dark" | "theme.sunny" | "theme.rain";
}> = [
  { value: "light", label_key: "theme.light" },
  { value: "dark", label_key: "theme.dark" },
  { value: "sunny", label_key: "theme.sunny" },
  { value: "rain", label_key: "theme.rain" },
];

export const LOCALE_OPTIONS: ReadonlyArray<{
  value: Locale;
  label_key: "language.zh" | "language.en";
}> = [
  { value: "zh", label_key: "language.zh" },
  { value: "en", label_key: "language.en" },
];
