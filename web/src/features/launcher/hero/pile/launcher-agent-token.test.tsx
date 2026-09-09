// INPUT: Agent或Room Token、完整目录名和精确点击命令。
// OUTPUT: Agent具名、可键盘操作；Room绝不变成导航按钮。
// POS: 物理Token的DOM语义回归。
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { SpotlightToken } from "@/types/app/launcher";
import { LauncherAgentToken } from "./launcher-agent-token";
const token: SpotlightToken = { key: "nova", name: "Nova Researcher", label: "NR", kind: "agent", agent_id: "agent-private", swatch: { fill: "#112233", ring: "#334455", text: "#ffffff" } };
const config = { key: "nova", angle: 0, delay: 0, radius: 20, size: 40, spawnX: 100, spawnY: -10 };
const wrap = (current: SpotlightToken, onSelectAgent = vi.fn()) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (_key, params) => `Chat with ${params?.name}` }}><LauncherAgentToken bindElement={vi.fn()} config={config} isActive onSelectAgent={onSelectAgent} token={current} /></I18N_CONTEXT.Provider>;
it("uses the full name and retains exact keyboard selection", async () => {
  const select = vi.fn(); render(wrap(token, select));
  const button = screen.getByRole("button", { name: "Chat with Nova Researcher" });
  expect(button.getAttribute("aria-pressed")).toBe("true");
  button.focus(); await userEvent.setup().keyboard("{Enter}");
  expect(select).toHaveBeenCalledExactlyOnceWith("agent-private");
});
it("keeps Room tokens decorative even if an Agent identity is present", () => {
  const { container } = render(wrap({ ...token, kind: "room" }));
  expect(screen.queryByRole("button")).toBeNull();
  expect(container.querySelector('[data-token-kind="room"]')?.getAttribute("aria-hidden")).toBe("true");
});
