// INPUT: Agent/Room identities, failed images and member mosaics.
// OUTPUT: Named avatars with recoverable image fallbacks and bounded mosaics.
// POS: Public Avatar behavior regressions; browser tests own actual geometry.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UiRoomAvatar } from "./avatar";

const members = Array.from({ length: 12 }, (_, index) => ({
  id: String(index), name: `Member ${index}`, avatar: null,
}));

describe("public avatars", () => {

  it("recovers a failed member picture to its initial inside the same room", () => {
    render(<UiRoomAvatar members={[{ id: "a", name: "Nova", avatar: "/missing.png" }, members[1]]} title="Room" />);
    const room = screen.getByRole("img", { name: "Room" });
    fireEvent.error(room.querySelector("img")!);
    expect(Array.from(room.children, (tile) => tile.textContent)).toContain("N");
    expect(screen.getAllByRole("img")).toHaveLength(1);
  });
});


it("keeps the room mosaic stable across reordered sources and applies changed pictures without mutating members", () => {
  const source = ["e", "c", "a", "d", "b"].map((id) => ({ id, name: id, avatar: `/${id}.png` }));
  const originalOrder = source.map((member) => member.id);
  const { rerender } = render(<UiRoomAvatar members={source} title="Shared room" />);
  const images = () => Array.from(screen.getByRole("img", { name: "Shared room" }).querySelectorAll("img"), (img) => img.getAttribute("src"));
  expect(images()).toEqual(["/a.png", "/b.png", "/c.png", "/d.png", "/e.png"]);
  rerender(<UiRoomAvatar members={[...source].reverse().map((member) => member.id === "c" ? { ...member, avatar: "/new-c.png" } : member)} title="Shared room" />);
  expect(images()).toEqual(["/a.png", "/b.png", "/new-c.png", "/d.png", "/e.png"]);
  expect(source.map((member) => member.id)).toEqual(originalOrder);
});
