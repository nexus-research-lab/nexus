import { UiQRCode } from "@/shared/ui/display/qr-code";

export function LoginQRCode({ payload }: { payload: string }) {
  return <UiQRCode alt="频道扫码登录二维码" payload={payload} />;
}
