import type { TranslationKey } from "../zh";
import { enCoreMessages } from "./core";
import { enNavigationMessages } from "./navigation";
import { enCapabilityMessages } from "./capability";
import { enSettingsMessages } from "./settings";
import { enConversationMessages } from "./conversation";
import { enAgentMessages } from "./agent";

export const enMessages = {
  ...enCoreMessages,
  ...enNavigationMessages,
  ...enCapabilityMessages,
  ...enSettingsMessages,
  ...enConversationMessages,
  ...enAgentMessages,
} satisfies Record<TranslationKey, string>;
