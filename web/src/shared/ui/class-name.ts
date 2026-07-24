import { type ClassValue, clsx } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/* 中文注释：自定义字号档位（2xs / compact / md）必须注册进 tailwind-merge，
   否则它与已知字号类冲突时不会被去重，最终大小由 CSS 源码顺序而非类顺序决定。 */
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": ["text-2xs", "text-compact", "text-md"],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
