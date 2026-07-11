import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button";
import { UiStateBlock } from "@/shared/ui/state-block";

export function RoomChatErrorView() {
  const { t } = useI18n();
  return (
    <div className="flex h-full min-h-80 items-center justify-center px-6 py-10">
      <UiStateBlock
        actions={(
          <UiButton
            onClick={() => window.location.reload()}
            size="md"
            tone="primary"
            variant="solid"
          >
            {t("common.refresh")}
          </UiButton>
        )}
        description={t("room.chat_render_error_description")}
        title={t("room.chat_render_error_title")}
        tone="danger"
      />
    </div>
  );
}
