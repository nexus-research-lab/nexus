// INPUT: 当前界面密度和 Provider 配置入口。
// OUTPUT: Provider 不可用提示及按需打开的配置弹窗。
// POS: Conversation Provider 恢复入口；不拥有通用提示或按钮视觉。

"use client";

import { useState } from "react";
import { AlertTriangle } from "lucide-react";

import { ProviderSetupDialog } from "@/features/onboarding/provider-setup/provider-setup-dialog";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";

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
        <UiInlineNotice
          action={{
            label: t("onboarding.provider_setup_action"),
            onClick: () => setSetupOpen(true),
          }}
          icon={<AlertTriangle />}
          message={t("onboarding.provider_setup_banner")}
          tone="warning"
        />
      </div>
      <ProviderSetupDialog
        isOpen={setupOpen}
        onClose={() => setSetupOpen(false)}
      />
    </>
  );
}
