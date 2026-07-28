/**
 * 判断当前浏览器是否适合启用 SVG 液态玻璃滤镜。
 *
 * 这里只负责运行时能力边界；位移图与高光图由离线脚本生成，
 * 避免组件挂载时重复创建 Canvas 资源。
 */
export function supportsTrueLiquidGlass(): boolean {
  if (typeof window === "undefined" || typeof CSS === "undefined" || typeof navigator === "undefined") {
    return false;
  }

  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    return false;
  }

  const supportsBackdrop = CSS.supports("backdrop-filter", "blur(1px)")
    || CSS.supports("-webkit-backdrop-filter", "blur(1px)");
  if (!supportsBackdrop) {
    return false;
  }

  const connection = navigator as Navigator & {
    connection?: { saveData?: boolean };
  };
  if (connection.connection?.saveData) {
    return false;
  }

  // Firefox 的 SVG 位移滤镜表现不稳定，其余浏览器交给能力检测决定。
  return !/Firefox\//i.test(navigator.userAgent);
}
