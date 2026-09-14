// INPUT: 含公式的聊天目录摘要和精确行操作。
// OUTPUT: 证明目录使用本地化公式标记，保留标题、普通摘要文字与行导航。
// POS: Home 侧栏实际消费入口的 DOM 回归，不验证像素尺寸。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { SidebarConversationItem } from "./sidebar-conversation-model";
import { ConversationRow } from "./sidebar-list-rows";

const item: SidebarConversationItem = {
  id: "conversation", isPinned: false, kind: "dm", title: "Nova",
  summary: String.raw`结合常数 \[K_D = \frac{k_d}{k_a}\] 的定义`,
  timeLabel: "11:13", members: [], lastActivityAt: 0, messageCount: 1,
  activityStatus: null, canDelete: false,
};

describe("sidebar formula previews", () => {
  it.each(["zh", "en"] as const)("keeps a compact summary and row action in %s", (locale) => {
    const onClick = vi.fn();
    const { container } = render(
      <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
        <ConversationRow isActive item={item} onClick={onClick} />
      </I18N_CONTEXT.Provider>,
    );
    const summary = container.querySelector(".nexus-sidebar-conversation-summary")!;
    expect(summary.textContent).toBe(`结合常数 ${locale === "zh" ? "[公式]" : "[Formula]"} 的定义`);
    expect(summary.querySelector(".katex, math, pre, [tabindex]")).toBeNull();
    expect(screen.getByText("11:13")).toBeTruthy();
    fireEvent.click(screen.getByText("Nova"));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
