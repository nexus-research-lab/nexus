"use client";

import { useState } from "react";
import { AlertTriangle } from "lucide-react";

import { ProviderSetupDialog } from "@/features/onboarding/provider-setup/provider-setup-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { cn } from "@/shared/ui/class-name";

import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "./conversation-panel-styles";

interface ProviderUnavailableBannerProps {
  compact?: boolean;
}

export function ProviderUnavailableBanner({ compact = false }: ProviderUnavailableBannerProps) {
  const { t } = useI18n();
  const [setupOpen, setSetupOpen] = useState(false);

  return (
    <>
      <div className={cn(
        compact
          ? "px-1 pb-1"
          : `${CONVERSATION_CONTENT_LANE_CLASS_NAME} px-4 pb-2 sm:px-6 xl:px-8`,
      )}>
        <div className="flex items-center gap-2 radius-control-md border border-[color:color-mix(in_srgb,var(--warning)_26%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] px-3 py-2 text-xs text-(--warning)">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
          <span className="min-w-0 flex-1">
            {t("onboarding.provider_setup_banner")}
          </span>
          <button
            className={getUiButtonClassName(
              { size: "xs", tone: "primary", variant: "text" },
              "shrink-0 px-1.5 font-medium",
            )}
            onClick={() => setSetupOpen(true)}
            type="button"
          >
            {t("onboarding.provider_setup_action")}
          </button>
        </div>
      </div>
      <ProviderSetupDialog
        isOpen={setupOpen}
        onClose={() => setSetupOpen(false)}
      />
    </>
  );
}
