// INPUT: 已上线 Connector 的品牌键、静态资产与统一品牌图标组件。
// OUTPUT: 证明高德线框标识和其他品牌统一经过公共单色品牌组件呈现。
// POS: Connector 品牌资产呈现策略的回归合同。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ConnectorIcon } from "./connector-icon";

describe("ConnectorIcon", () => {
  it("renders the Amap outline through the shared monochrome treatment", () => {
    render(<ConnectorIcon icon="amap" size="lg" title="高德地图" />);

    const frame = screen.getByLabelText("高德地图");
    const mark = frame.firstElementChild as HTMLElement;

    expect(frame.className).toContain("surface-radius-md");
    expect(mark.style.maskImage).toContain("/icon/connector/amap.svg");
    expect(mark.style.backgroundColor).toBe("var(--text-strong)");
    expect(frame.querySelector("img")).toBeNull();
  });
});
