"use client";

import { useProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { useAuth } from "@/shared/auth/auth-context";

import type { ConversationPanelEnvironment } from "./conversation-panel-model";

export function useConversationPanelEnvironment(
  layout: "desktop" | "mobile",
): ConversationPanelEnvironment {
  const { status } = useAuth();
  const { hasAvailableProvider, isReady } = useProviderAvailability();
  return {
    currentUserAvatar: status?.avatar ?? null,
    isMobileLayout: layout === "mobile",
    providerWarningVisible: isReady && !hasAvailableProvider,
  };
}
