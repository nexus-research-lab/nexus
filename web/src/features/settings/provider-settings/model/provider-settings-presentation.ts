import type { ProviderApiFormat } from "@/types/capability/provider";

export type SettingsTabKey = "providers";

export const SETTINGS_TABS: {
  key: SettingsTabKey;
  labelKey: "settings.tabs.providers";
}[] = [
  { key: "providers", labelKey: "settings.tabs.providers" },
];

export const PROVIDER_LABEL_CLASS_NAME =
  "text-[13px] font-semibold text-(--text-strong)";

export const API_FORMAT_LABELS: Record<ProviderApiFormat, string> = {
  chat_completions: "Chat Completions (/chat/completions)",
  responses: "Responses (/responses)",
  anthropic_messages: "Anthropic Messages (/v1/messages)",
  openai_image_generation: "OpenAI Image Generation (/images/generations)",
  dashscope_image_generation: "DashScope Image Generation",
  modelscope_image_generation: "ModelScope Image Generation",
};

export const API_FORMAT_SHORT_LABELS: Record<ProviderApiFormat, string> = {
  chat_completions: "Completions",
  responses: "Responses",
  anthropic_messages: "Anthropic",
  openai_image_generation: "OpenAI Image",
  dashscope_image_generation: "DashScope Image",
  modelscope_image_generation: "ModelScope Image",
};

export function formatTokenPreview(
  maskedToken: string | null | undefined,
  emptyLabel: string,
): string {
  return maskedToken?.trim() || emptyLabel;
}

export function formatCount(value?: number | null): string {
  if (!value || value <= 0) {
    return "auto";
  }
  if (value >= 1000) {
    return `${Math.round(value / 1000)}K`;
  }
  return String(value);
}
