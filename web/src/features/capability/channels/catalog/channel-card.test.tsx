// INPUT: Channel entry and configuration callback.
// OUTPUT: Documentation destination and configuration action.
// POS: Channel card interaction test.

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ChannelConfigView } from "@/lib/api/capability/channel-api";

import { ChannelCard } from "./channel-card";

vi.mock("@/shared/i18n/i18n-context", () => ({
  useI18n: () => ({ locale: "zh", t: (key: string) => key }),
}));

const CHANNEL: ChannelConfigView = {
  channel_type: "telegram",
  title: "Telegram",
  bot_label: "Bot Token",
  description: "Connect a Telegram bot.",
  docs_url: "https://core.telegram.org/bots",
  runtime_status: "ready",
  supports_group: true,
  supports_qr_code: false,
  supports_oauth_link: false,
  capabilities: ["text"],
  credential_fields: [],
  configured: true,
  connection_state: "connected",
  status: "active",
  agent_id: "agent-1",
  agent_name: "Atlas",
  has_credentials: true,
  stats: {
    paired_user_count: 2,
    paired_group_count: 1,
    pending_count: 0,
  },
};

describe("ChannelCard", () => {
  it("opens documentation and dispatches configuration", async () => {
    const user = userEvent.setup();
    const onConfigure = vi.fn();
    render(
<ChannelCard item={CHANNEL} onConfigure={onConfigure} />
);

    const docs = screen.getByRole("link", {
      name: "capability.channel_docs_action",
    });
    expect(docs.getAttribute("href")).toBe(CHANNEL.docs_url);

    await user.click(screen.getByRole("button", {
      name: "capability.channel_configure_action",
    }));
    expect(onConfigure).toHaveBeenCalledOnce();
  });
});
