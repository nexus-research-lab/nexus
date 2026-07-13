import { useEffect, useId, useState } from "react";

import { supportsTrueLiquidGlass } from "./liquid-glass-engine";

/**
 * 首次渲染统一走降级材质，挂载后再启用浏览器滤镜，避免宿主能力参与初始树结构。
 */
export function useSupportsTrueLiquidGlass(): boolean {
  const [supported, setSupported] = useState(false);

  useEffect(() => {
    setSupported(supportsTrueLiquidGlass());
  }, []);

  return supported;
}

/** SVG filter id 只需在当前 React 根内稳定且唯一。 */
export function useLiquidGlassFilterId(prefix: string): string {
  const id = useId();
  return `${prefix}-${id.replaceAll(":", "")}`;
}
