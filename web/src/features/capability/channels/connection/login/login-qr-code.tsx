// INPUT: Opaque Channel login QR payload.
// OUTPUT: Scannable QR without exposing the underlying token or URL as text.
// POS: Channel login QR presentation boundary.
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiQRCode } from "@/shared/ui/display/qr-code";
import { FeedbackBanner } from "@/shared/ui/feedback/feedback-banner";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function LoginQRCode({
  payload,
  required,
}: {
  payload: string;
  required: boolean;
}) {
  const { t } = useI18n();
  if (!payload.trim()) {
    return required ? (
      <FeedbackBanner
        impact={t("capability.channel_login_qr_missing_impact")}
        nextStep={t("capability.channel_login_qr_missing_next_step")}
        title={t("capability.channel_login_qr_missing_title")}
        tone="warning"
      />
    ) : null;
  }
  return (
    <UiQRCode
      alt="频道扫码登录二维码"
      failureFallback={(
        <div className="space-y-1.5 text-left">
          <p className={getUiTypographyClassName({
            role: "metadata",
            tone: "strong",
            weight: "semibold",
          })}>
            {t("capability.channel_login_qr_failed_title")}
          </p>
          <p className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>
            {t("capability.channel_login_qr_failed_message")}
          </p>
          <p className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>
            {t("capability.channel_login_qr_failed_impact")}
          </p>
          <p className={getUiTypographyClassName({
            role: "metadata",
            tone: "default",
            weight: "medium",
          })}>
            {t("capability.channel_login_qr_failed_next_step")}
          </p>
        </div>
      )}
      loadingLabel={t("capability.channel_login_qr_loading")}
      payload={payload}
      showPayload={false}
    />
  );
}
