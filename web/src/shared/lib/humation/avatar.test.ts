import { expect, it } from "vitest";
import { avatarAssets, createAvatarState, decodeAvatar, encodeAvatar, getHumationAvatarSrc } from "./avatar";
import { getIconAvatarSrc } from "@/lib/avatar";

it("round-trips stable selections, colors and Nexus parts through the existing avatar field", () => {
  const state = createAvatarState("agent-123");
  expect(state).toEqual(createAvatarState("agent-123"));
  for (const part of avatarAssets.parts) {
    const customized = { ...state, selections: { ...state.selections, [part.selectionSlot]: part.id } };
    const value = encodeAvatar(customized);
    expect(value.length).toBeLessThanOrEqual(255);
    expect(decodeAvatar(value)).toEqual(customized);
    const src = getIconAvatarSrc(value);
    expect(src).toBe(getHumationAvatarSrc(value));
    expect(decodeURIComponent(src!)).toContain(`data-hm-part-id="${part.id}"`);
  }
});
it("rejects malformed colors, wrong slots, unknown versions and oversized input", () => {
  const valid = encodeAvatar(createAvatarState("test"));
  for (const value of ["h1:", valid.replace("h1:", "h2:"), valid + ".extra", valid.replace(/\.[A-F0-9]{6}$/, '.<svg>'), 'h1:' + 'a'.repeat(256), valid.replace(/hm1-p-\d+/, "unknown")]) {
    expect(decodeAvatar(value)).toBeNull();
  }
  expect(getIconAvatarSrc("h1:bad")).toBeNull();
  expect(getIconAvatarSrc("5")).toBe("/icon/agent/5.png");
});


it("varies background by seed and preserves it in saved SVG avatars", () => {
  const backgrounds = new Set<string>();
  for (let index = 0; index < 32; index += 1) {
    const seed = `background-${index}`;
    const state = createAvatarState(seed);
    backgrounds.add(state.background);
    expect(createAvatarState(seed).background).toBe(state.background);
    const saved = encodeAvatar(state);
    expect(decodeAvatar(saved)?.background).toBe(state.background);
    expect(decodeURIComponent(getHumationAvatarSrc(saved)!)).toContain(`fill="#${state.background}"`);
  }
  expect(backgrounds.size).toBe(8);
});
