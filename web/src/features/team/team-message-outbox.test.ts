import { beforeEach, expect, it } from "vitest";
import { TeamMessageOutbox } from "./team-message-outbox";

beforeEach(() => localStorage.clear());

it("persists attachment-only intents without replacing file references on retry", () => {
  const box = new TeamMessageOutbox("org:user:room");
  const intent = {id:"attachment",text:"",agentIds:["agent"],membershipVersion:1,attachments:[{id:"file",name:"report.txt",size:3,sha256:"a".repeat(64)}]};
  box.save(intent);
  expect(box.read()).toEqual([intent]);
  box.confirm(intent.id);
  expect(box.read()).toEqual([]);
});

it("isolates accounts and preserves independent window intents until exact confirmation", () => {
  const first = new TeamMessageOutbox("org:user:room");
  const second = new TeamMessageOutbox("org:user:room");
  const other = new TeamMessageOutbox("org:other:room");
  first.save({id: "one", text: "one", agentIds: ["agent"], membershipVersion: 2});
  second.save({id: "two", text: "two", agentIds: [], membershipVersion: 3});
  expect(first.read().map((value) => value.id).sort()).toEqual(["one", "two"]);
  expect(other.read()).toEqual([]);
  second.confirm("one");
  expect(first.read().map((value) => value.id)).toEqual(["two"]);
  const key = localStorage.key(0)!;
  localStorage.setItem(key, "broken");
  expect(() => first.read()).toThrow();
  expect(localStorage.getItem(key)).toBe("broken");
});
