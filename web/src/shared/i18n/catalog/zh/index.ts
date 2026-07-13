import { zhCoreMessages } from "./core";
import { zhNavigationMessages } from "./navigation";
import { zhCapabilityMessages } from "./capability";
import { zhSettingsMessages } from "./settings";
import { zhConversationMessages } from "./conversation";
import { zhAgentMessages } from "./agent";

export const zhMessages = {
  ...zhCoreMessages,
  ...zhNavigationMessages,
  ...zhCapabilityMessages,
  ...zhSettingsMessages,
  ...zhConversationMessages,
  ...zhAgentMessages,
} as const;

export type TranslationKey = keyof typeof zhMessages;
