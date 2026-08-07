"use client";

import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";

import type { ConversationPanelEnvironment } from "./conversation-panel-model";

export function useConversationPanelEnvironment(
  layout: "desktop" | "mobile",
): ConversationPanelEnvironment {
  const { hasAvailableProvider, isReady } = useProviderAvailability();
  return {
    isMobileLayout: layout === "mobile",
    providerWarningVisible: isReady && !hasAvailableProvider,
  };
}
