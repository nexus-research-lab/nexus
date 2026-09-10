// INPUT: 当前设置模块、搜索目标文案键与内容容器。
// OUTPUT: 页面就绪后将匹配设置项滚动到视口中央。
// POS: 设置搜索定位边界；只查找内容区，不操作设置控件或修改配置。
import { useEffect, type RefObject } from "react";
import { useLocation, useSearchParams } from "react-router-dom";
import { useI18n } from "@/shared/i18n/i18n-context";
import { SETTINGS_SEARCH_ITEMS } from "./settings-search-items";
import type { SettingsSectionKey } from "./settings-navigation-model";

export function useSettingsSearchTarget(
  contentRef: RefObject<HTMLDivElement | null>,
  section: SettingsSectionKey,
) {
  const { t } = useI18n();
  const { key: navigationKey } = useLocation();
  const [params] = useSearchParams();
  const target = params.get("target");
  useEffect(() => {
    const content = contentRef.current;
    const entry = SETTINGS_SEARCH_ITEMS[section].find((fields) => fields[0] === target);
    if (!content || !entry) return;
    const label = t(entry[0]);
    // 复用页面实际渲染的标签，避免每个设置控件再维护一份锚点标识。
    const locate = () => {
      const elements = Array.from(content.querySelectorAll<HTMLElement>("*"));
      const element = elements.find((candidate) =>
        candidate.textContent?.trim() === label
        && !Array.from(candidate.children).some((child) => child.textContent?.trim() === label),
      ) ?? elements.find((candidate) => candidate.getAttribute("aria-label") === label);
      if (!element) return false;
      element.scrollIntoView({ block: "center", behavior: "instant" });
      return true;
    };
    if (locate()) return;
    // 资料、供应商等模块可能先显示加载态；内容到达后完成一次定位。
    const observer = new MutationObserver(() => {
      if (locate()) observer.disconnect();
    });
    observer.observe(content, { childList: true, subtree: true, characterData: true });
    return () => observer.disconnect();
  }, [contentRef, navigationKey, section, t, target]);
}
