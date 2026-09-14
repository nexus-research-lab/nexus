// INPUT: 飞书 Device Flow 阶段与手工凭据草稿。
// OUTPUT: 当前用户动作所需的短标题、状态、二维码/跳转模式和完整性判断。
// POS: 飞书连接弹窗的纯展示模型，不携带教程式副标题。
import type { TranslationKey } from "@/shared/i18n/messages";
import type {
  ConnectorDeviceAuthStart,
  ConnectorDeviceAuthStage,
} from "@/types/capability/connector";

export interface FeishuDeviceAuthPresentation {
  actionLabel: TranslationKey;
  qrAlt?: TranslationKey;
  initialMessage: TranslationKey;
  showQRCode: boolean;
  title: TranslationKey;
}

const FEISHU_DEVICE_AUTH_PRESENTATION: Record<
  ConnectorDeviceAuthStage,
  FeishuDeviceAuthPresentation
> = {
  app_selection: {
    actionLabel: "capability.connector_flow_feishu_open",
    qrAlt: "capability.connector_flow_feishu_qr",
    initialMessage: "capability.connector_flow_wait_app",
    showQRCode: true,
    title: "capability.connector_flow_select_app",
  },
  user_authorization: {
    actionLabel: "capability.connector_flow_continue_auth",
    initialMessage: "capability.connector_flow_waiting",
    showQRCode: false,
    title: "capability.connector_flow_feishu_title",
  },
};

export function getFeishuDeviceAuthPresentation(
  stage?: ConnectorDeviceAuthStage,
): FeishuDeviceAuthPresentation {
  return FEISHU_DEVICE_AUTH_PRESENTATION[
    stage ?? "user_authorization"
  ];
}

export function feishuManualCredentialsComplete(
  clientId: string,
  clientSecret: string,
): boolean {
  return Boolean(clientId.trim() && clientSecret.trim());
}

export function shouldAutoOpenFeishuUserAuthorization(
  session: ConnectorDeviceAuthStart | null,
): boolean {
  return Boolean(
    session?.connector_id === "feishu-docx"
    && session.stage === "user_authorization",
  );
}
