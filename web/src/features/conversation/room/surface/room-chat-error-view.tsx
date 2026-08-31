import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";

export function RoomChatErrorView() {
  const { t } = useI18n();
  return (
    <div className="flex h-full min-h-80 items-center justify-center px-6 py-10">
      <UiResourceState
        description={t("room.chat_render_error_description")}
        impact={t("room.chat_render_error_impact")}
        nextStep={t("room.chat_render_error_next_step")}
        primaryAction={{
          label: t("common.refresh"),
          onClick: () => window.location.reload(),
        }}
        state="error"
        title={t("room.chat_render_error_title")}
        urgency="polite"
      />
    </div>
  );
}
