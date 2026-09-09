// INPUT: 真实设置路由、搜索目标及延迟加载的设置内容。
// OUTPUT: 验证同模块搜索点击可定位，并等待异步内容。
// POS: 搜索路由到页面定位的回归测试，不加载真实设置服务。
import { useRef, useState } from "react";
import { MemoryRouter } from "react-router-dom";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { MESSAGES, type TranslationKey } from "@/shared/i18n/messages";
import { useSettingsNavigation } from "./use-settings-navigation";
import { useSettingsSearchTarget } from "./use-settings-search-target";

vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: TranslationKey) => MESSAGES.zh[key] }) }));

it("scrolls after content loads and on repeated clicks within the current section", async () => {
  const scroll = vi.fn();
  const previous = HTMLElement.prototype.scrollIntoView;
  HTMLElement.prototype.scrollIntoView = scroll;
  function Page() {
    const ref = useRef<HTMLDivElement>(null);
    const [loaded, setLoaded] = useState(false);
    const { activeSection, selectSection } = useSettingsNavigation();
    useSettingsSearchTarget(ref, activeSection);
    return <>
      <button onClick={() => selectSection("general", "settings.general.auto_memory_title")}>搜索结果</button>
      <button onClick={() => setLoaded(true)}>加载完成</button>
      <div ref={ref}>{loaded ? <h3>自动记忆</h3> : null}</div>
    </>;
  }
  try {
    const user = userEvent.setup();
    render(<MemoryRouter initialEntries={["/settings?section=general"]}><Page /></MemoryRouter>);
    await user.click(screen.getByText("搜索结果"));
    expect(scroll).not.toHaveBeenCalled();
    await user.click(screen.getByText("加载完成"));
    await waitFor(() => expect(scroll).toHaveBeenCalledTimes(1));
    await user.click(screen.getByText("搜索结果"));
    await waitFor(() => expect(scroll).toHaveBeenCalledTimes(2));
    expect(scroll).toHaveBeenLastCalledWith({ block: "center", behavior: "instant" });
  } finally {
    HTMLElement.prototype.scrollIntoView = previous;
  }
});
