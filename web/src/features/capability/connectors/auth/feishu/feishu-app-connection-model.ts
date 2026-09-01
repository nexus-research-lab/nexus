// INPUT: 飞书 Device Flow 阶段与手工凭据草稿。
// OUTPUT: 当前用户动作所需的短标题、状态、二维码/跳转模式和完整性判断。
// POS: 飞书连接弹窗的纯展示模型，不携带教程式副标题。
import type {
  ConnectorDeviceAuthStart,
  ConnectorDeviceAuthStage,
} from "@/types/capability/connector";

export interface FeishuDeviceAuthPresentation {
  actionLabel: string;
  qrAlt?: string;
  initialMessage: string;
  showQRCode: boolean;
  title: string;
}

const FEISHU_DEVICE_AUTH_PRESENTATION: Record<
  ConnectorDeviceAuthStage,
  FeishuDeviceAuthPresentation
> = {
  app_selection: {
    actionLabel: "打开飞书",
    qrAlt: "飞书应用选择二维码",
    initialMessage: "等待选择应用",
    showQRCode: true,
    title: "选择飞书应用",
  },
  user_authorization: {
    actionLabel: "继续授权",
    initialMessage: "等待授权",
    showQRCode: false,
    title: "连接飞书云文档",
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
