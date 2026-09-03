// INPUT: 全部 Channel 类型及目录/弹窗尺寸。
// OUTPUT: 证明每个平台使用独立品牌资源，且统一为公共单色中性图标容器。
// POS: Channel 品牌身份 DOM 合同；平台连接状态和目录行为由各自测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ImChannelType } from "@/lib/api/capability/channel-api";

import { ChannelIcon } from "./channel-icon";

const CHANNELS: ReadonlyArray<{ label: string; type: ImChannelType }> = [
  { label: "钉钉", type: "dingtalk" },
  { label: "企业微信", type: "wechat" },
  { label: "微信", type: "weixin-personal" },
  { label: "飞书", type: "feishu" },
  { label: "Telegram", type: "telegram" },
  { label: "Discord", type: "discord" },
];

describe("ChannelIcon", () => {
  it("maps every platform to a distinct monochrome brand mask", () => {
    const { container } = render(
      <>{CHANNELS.map((channel) => (
        <ChannelIcon key={channel.type} type={channel.type} />
      ))}</>,
    );

    const maskImages = CHANNELS.map(({ label }) => {
      const frame = screen.getByLabelText(label);
      expect(frame.className).toContain("bg-(--surface-panel-background)");
      expect(frame.className).toContain("radius-control-sm");
      const mark = frame.firstElementChild as HTMLElement;
      expect(mark.style.backgroundColor).toBe("var(--text-strong)");
      return mark.style.maskImage;
    });

    expect(new Set(maskImages).size).toBe(CHANNELS.length);
    expect(container.innerHTML).not.toMatch(/#[0-9a-f]{3,8}|text-white/i);
  });

  it("uses the same large capability frame in connection dialogs", () => {
    render(<ChannelIcon size="dialog" type="telegram" />);

    const frame = screen.getByLabelText("Telegram");
    expect(frame.className).toContain("h-14");
    expect(frame.className).toContain("surface-radius-md");
  });
});
