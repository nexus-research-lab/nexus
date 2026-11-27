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
