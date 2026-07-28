/**
 * INPUT: 浏览器能力、动效偏好、节流状态与 user agent。
 * OUTPUT: 当前宿主是否适合启用 SVG 液态玻璃滤镜。
 * POS: 液态玻璃组件共用的运行时能力边界；资产仍由离线脚本生成。
 */
interface LiquidGlassCapability {
  prefersReducedMotion: boolean;
  saveData: boolean;
  supportsBackdrop: boolean;
  userAgent: string;
}

const SAFARI_USER_AGENT = /Safari\//i;
const SAFARI_COMPATIBILITY_USER_AGENT =
  /(?:Chrome|Chromium|CriOS|Edg|EdgiOS|OPR|OPiOS|Firefox|FxiOS)\//i;

export function isSafariUserAgent(userAgent: string): boolean {
  return SAFARI_USER_AGENT.test(userAgent)
    && !SAFARI_COMPATIBILITY_USER_AGENT.test(userAgent);
}

export function canUseTrueLiquidGlass({
  prefersReducedMotion,
  saveData,
  supportsBackdrop,
  userAgent,
}: LiquidGlassCapability): boolean {
  if (prefersReducedMotion || saveData || !supportsBackdrop) {
    return false;
  }

  // Firefox 的位移滤镜表现不稳定；Safari 卸载 SVG backdrop-filter 后可能残留合成层像素。
  return !/Firefox\//i.test(userAgent) && !isSafariUserAgent(userAgent);
}

export function supportsTrueLiquidGlass(): boolean {
  if (
    typeof window === "undefined"
    || typeof CSS === "undefined"
    || typeof navigator === "undefined"
  ) {
    return false;
  }

  const connection = navigator as Navigator & {
    connection?: { saveData?: boolean };
  };

  return canUseTrueLiquidGlass({
    prefersReducedMotion: window.matchMedia(
      "(prefers-reduced-motion: reduce)",
    ).matches,
    saveData: connection.connection?.saveData === true,
    supportsBackdrop:
      CSS.supports("backdrop-filter", "blur(1px)")
      || CSS.supports("-webkit-backdrop-filter", "blur(1px)"),
    userAgent: navigator.userAgent,
  });
}
