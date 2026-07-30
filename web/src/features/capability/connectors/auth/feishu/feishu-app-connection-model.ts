import type {
  ConnectorDeviceAuthStart,
  ConnectorDeviceAuthStage,
} from "@/types/capability/connector";

export interface FeishuDeviceAuthPresentation {
  actionLabel: string;
  qrAlt?: string;
  initialMessage: string;
  showQRCode: boolean;
  subtitle: string;
  title: string;
}

const FEISHU_DEVICE_AUTH_PRESENTATION: Record<
  ConnectorDeviceAuthStage,
  FeishuDeviceAuthPresentation
> = {
  app_selection: {
    actionLabel: "打开飞书",
    qrAlt: "飞书应用选择二维码",
    initialMessage: "等待飞书选择或创建应用",
    showQRCode: true,
    subtitle: "使用飞书扫码，在官方页面选择已有应用或创建新应用并补齐云文档权限。",
    title: "选择或创建飞书应用",
  },
  user_authorization: {
    actionLabel: "继续飞书授权",
    initialMessage: "等待当前用户授予云文档权限",
    showQRCode: false,
    subtitle: "应用已经确定，Nexus 将打开飞书页面完成当前用户的云文档授权。",
    title: "授权飞书云文档",
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
