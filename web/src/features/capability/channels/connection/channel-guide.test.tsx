// INPUT: Channel tutorial selection and live locale changes.
// OUTPUT: Preserves steps, protocol literals, safe links and disclosure state across languages.
// POS: Connection guide localization regression.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import type { ChannelConfigView, ImChannelType } from "@/lib/api/capability/channel-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { ChannelGuide } from "./channel-guide";

const cases: Array<[ImChannelType, number, string, string[]]> = [
  ["dingtalk", 6, "https://open.dingtalk.com/", ["Stream", "Webhook", "Client ID", "Client Secret", "sessionWebhook", "openConversationId", "Robot Code"]],
  ["discord", 4, "https://discord.com/developers/applications", ["New Application", "Application ID", "Reset Token", "Bot Token", "OAuth Client Secret", "Message Content Intent"]],
  ["feishu", 6, "https://open.feishu.cn/", ["App ID", "App Secret", "im.message.receive_v1", "im.message.reaction.created_v1", "Encrypt Key", "Verification Token"]],
  ["telegram", 4, "https://t.me/BotFather", ["@BotFather", "/newbot", "Bot Token"]],
  ["wechat", 5, "https://developer.work.weixin.qq.com/", ["Bot ID", "Secret", "stream"]],
  ["weixin-personal", 5, "", ["iLink Bot API", "ilink_bot_token", "getupdates", "sendmessage"]],
];
it.each(cases)("preserves %s tutorial contracts while switching language", async (channel_type, count, url, protocols) => {
  const user = userEvent.setup();
  const item = { channel_type } as ChannelConfigView;
  const show = (locale: "zh" | "en") => (
    <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
      <ChannelGuide item={item} runtimeNote="Server-provided runtime note" />
    </I18N_CONTEXT.Provider>
  );
  const view = render(show("zh"));
  const disclosure = view.container.querySelector("details")!;
  expect(disclosure.open).toBe(false);
  await user.click(screen.getByText("连接说明"));
  expect(disclosure.open).toBe(true);
  expect(screen.getAllByRole("listitem")).toHaveLength(count);
  protocols.forEach((value) => expect(disclosure.textContent).toContain(value));
  view.rerender(show("en"));
  expect(disclosure.open).toBe(true);
  expect(screen.getByText("Connection instructions")).toBeTruthy();
  expect(screen.getAllByRole("listitem")).toHaveLength(count);
  protocols.forEach((value) => expect(disclosure.textContent).toContain(value));
  expect(disclosure.textContent).not.toMatch(/[\p{Script=Han}]|\*\*|\{link\}/u);
  expect(screen.getByText("Server-provided runtime note")).toBeTruthy();
  if (url) {
    const link = screen.getByRole("link");
    expect(link.getAttribute("href")).toBe(url);
    expect(link.getAttribute("rel")).toBe("noopener noreferrer");
    expect(link.getAttribute("target")).toBe("_blank");
  } else {
    expect(screen.queryByRole("link")).toBeNull();
  }
});
