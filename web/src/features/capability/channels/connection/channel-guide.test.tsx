// INPUT: Channel tutorial selection and live locale changes.
// OUTPUT: Preserves steps, protocol literals, safe links and disclosure state across languages.
// POS: Connection guide localization regression.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import type { ChannelConfigView } from "@/lib/api/capability/channel-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { ChannelGuide } from "./channel-guide";


it("links the platform documentation inside the guide when the channel provides one", async () => {
  const user = userEvent.setup();
  const item = { channel_type: "telegram", docs_url: "https://core.telegram.org/bots" } as ChannelConfigView;
  render(
    <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}>
      <ChannelGuide item={item} />
    </I18N_CONTEXT.Provider>,
  );
  await user.click(screen.getByText("Connection instructions"));
  const docs = screen.getByRole("link", { name: "View setup documentation" });
  expect(docs.getAttribute("href")).toBe(item.docs_url);
  expect(docs.getAttribute("rel")).toBe("noopener noreferrer");
});
