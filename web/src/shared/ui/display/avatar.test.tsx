// INPUT: Agent/Room identities, failed images and member mosaics.
// OUTPUT: Named avatars with recoverable image fallbacks and bounded mosaics.
// POS: Public Avatar behavior regressions; browser tests own actual geometry.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UiAgentAvatar, UiRoomAvatar } from "./avatar";

const members = Array.from({ length: 12 }, (_, index) => ({
  id: String(index), name: `Member ${index}`, avatar: null,
}));

describe("public avatars", () => {
  it("keeps one accessible identity when an image fails, and retries a changed source", () => {
    const { rerender } = render(<UiAgentAvatar avatar="/missing.png" name="Maya Chen" />);
    const avatar = screen.getByRole("img", { name: "Maya Chen" });
    fireEvent.error(avatar.querySelector("img")!);
    expect(avatar.textContent).toBe("MC");
    expect(avatar.querySelector("img")).toBeNull();
    rerender(<UiAgentAvatar avatar="/missing.png" name="Nora Smith" />);
    expect(screen.getByRole("img", { name: "Nora Smith" }).textContent).toBe("NS");
    rerender(<UiAgentAvatar avatar="/replacement.png" name="Nora Smith" />);
    expect(screen.getByRole("img", { name: "Nora Smith" }).querySelector("img")?.getAttribute("src"))
      .toBe("/replacement.png");
    expect(screen.getAllByRole("img")).toHaveLength(1);
  });

  it.each(["xxs", "xs"] as const)("exposes a complete Unicode identity in a %s initial", (size) => {
    render(<UiAgentAvatar name="👩‍💻 Nova" size={size} />);
    expect(screen.getByRole("img", { name: "👩‍💻 Nova" }).textContent).toBe("👩‍💻");
  });

  it("names a room once and bounds even an excessive member limit to nine tiles", () => {
    render(<UiRoomAvatar maxMembers={12} members={members} title="Planning room" />);
    const room = screen.getByRole("img", { name: "Planning room" });
    expect(screen.getAllByRole("img")).toHaveLength(1);
    expect(room.children).toHaveLength(9);
    expect(Array.from(room.children, (tile) => tile.textContent)).toEqual(Array(9).fill("M"));
  });

  it("retains caller limits and the two-member overlap without duplicating identities", () => {
    const { rerender } = render(<UiRoomAvatar maxMembers={4} members={members} title="Room" />);
    expect(screen.getByRole("img", { name: "Room" }).children).toHaveLength(4);
    rerender(<UiRoomAvatar members={members.slice(0, 2)} title="Room" />);
    const room = screen.getByRole("img", { name: "Room" });
    expect(room.children).toHaveLength(2);
    expect(Array.from(room.children, (tile) => tile.textContent)).toEqual(["M", "M"]);
  });

  it("recovers a failed member picture to its initial inside the same room", () => {
    render(<UiRoomAvatar members={[{ id: "a", name: "Nova", avatar: "/missing.png" }, members[1]]} title="Room" />);
    const room = screen.getByRole("img", { name: "Room" });
    fireEvent.error(room.querySelector("img")!);
    expect(room.firstElementChild?.textContent).toBe("N");
    expect(screen.getAllByRole("img")).toHaveLength(1);
  });

  it("preserves a stable empty-room picture and offers a named fallback on failure", () => {
    const { rerender } = render(<UiRoomAvatar members={[]} roomId="stable-room" title="Before" />);
    const source = screen.getByRole("img", { name: "Before" }).querySelector("img")!.getAttribute("src");
    rerender(<UiRoomAvatar members={[]} roomId="stable-room" title="After" />);
    const room = screen.getByRole("img", { name: "After" });
    expect(room.querySelector("img")!.getAttribute("src")).toBe(source);
    fireEvent.error(room.querySelector("img")!);
    expect(room.querySelector("img")).toBeNull();
    expect(room.querySelector("svg")).not.toBeNull();
  });
});
