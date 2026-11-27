import { describe, expect, it } from "vitest";

import type { Message, UserMessage } from "@/types/conversation/message/entity";

import {
  replaceOptimisticUserMessage,
  type UserMessageAck,
} from "../runtime/model/conversation-runtime-reconciliation";
import {
  mergeLoadedMessages,
  upsertMessage,
  upsertRealtimeMessage,
} from "./message-collection-model";

type ChatKind = "dm" | "room";

const SESSION_KEY = "session-1";

/** One accepted input as each backend channel reports it. */
interface AcceptedInput {
  ack: UserMessageAck;
  history: UserMessage;
  live: UserMessage;
  optimistic: UserMessage;
}

function userRow(overrides: Partial<UserMessage>): UserMessage {
  return {
    agent_id: "",
    content: "hello",
    message_id: "row",
    role: "user",
    round_id: "round",
    session_key: SESSION_KEY,
    timestamp: 1,
    ...overrides,
  };
}

/**
 * DM persists `client_message_id` in history and omits it on the live round
 * marker; Room carries it on the live broadcast only and never persists it.
 */
function acceptedInput(
  kind: ChatKind,
  index: number,
  content = `input ${index}`,
): AcceptedInput {
  const clientMessageId = `local-${index}`;
  const inputSeq = 1_700_000_000_000_000 + index;
  const durable = userRow({
    content,
    input_seq: inputSeq,
    message_id: `msg-user-${index}`,
    round_id: `round-${index}`,
    timestamp: index * 10 + 1,
  });
  return {
    ack: {
      client_message_id: clientMessageId,
      input_seq: inputSeq,
      round_id: durable.round_id,
      user_message_committed: true,
      user_message_id: durable.message_id,
    },
    history: kind === "dm"
      ? { ...durable, client_message_id: clientMessageId }
      : durable,
    live: kind === "room"
      ? { ...durable, client_message_id: clientMessageId }
      : durable,
    optimistic: userRow({
      client_message_id: clientMessageId,
      content,
      message_id: clientMessageId,
      round_id: clientMessageId,
      timestamp: index * 10,
    }),
  };
}

const send = (messages: Message[], input: AcceptedInput) =>
  upsertMessage(messages, input.optimistic);
const ack = (messages: Message[], input: AcceptedInput) =>
  replaceOptimisticUserMessage(messages, input.ack);
const live = (messages: Message[], input: AcceptedInput) =>
  upsertRealtimeMessage(messages, input.live);
const refresh = (messages: Message[], ...inputs: AcceptedInput[]) =>
  mergeLoadedMessages(inputs.map((input) => input.history), messages);

function userRows(messages: Message[]) {
  return messages.flatMap((message) => (
    message.role === "user"
      ? [{ content: message.content, input_seq: message.input_seq }]
      : []
  ));
}

function expectedRows(...inputs: AcceptedInput[]) {
  return inputs.map((input) => ({
    content: input.optimistic.content,
    input_seq: input.ack.input_seq,
  }));
}

describe.each<ChatKind>(["dm", "room"])("%s user rows keyed by input_seq", (kind) => {
  const first = acceptedInput(kind, 1);
  const second = acceptedInput(kind, 2);

  it("keeps one row when a refresh returns the acknowledged input", () => {
    const settled = ack(send([], first), first);

    expect(userRows(refresh(settled, first))).toEqual(expectedRows(first));
    expect(refresh(settled, first)[0]).toMatchObject({
      client_message_id: first.ack.client_message_id,
    });
    expect(userRows(refresh([], first))).toEqual(expectedRows(first));
  });

  it("keeps one row when a reconnect replays the live row and the acknowledgement", () => {
    const settled = ack(live(send([], first), first), first);
    const replayed = ack(live(refresh(settled, first), first), first);

    expect(userRows(replayed)).toEqual(expectedRows(first));
  });

  it("settles to one row when the acknowledgement never arrives and the refresh lands before the replayed live row", () => {
    const refreshed = refresh(send([], first), first);
    const replayed = live(refreshed, first);

    expect(userRows(kind === "room" ? replayed : refreshed)).toEqual(expectedRows(first));
    expect(userRows(live(replayed, first))).toEqual(expectedRows(first));
  });

  it("does not drop an acknowledged row that a stale refresh has not caught up with", () => {
    const settled = ack(send(ack(send([], first), first), second), second);

    expect(userRows(refresh(settled, first))).toEqual(expectedRows(first, second));
  });

  it("keeps two inputs with the same text as two rows", () => {
    const repeat = acceptedInput(kind, 3, first.optimistic.content);
    const settled = live(ack(send(live(ack(send([], first), first), first), repeat), repeat), repeat);

    expect(userRows(settled)).toEqual(expectedRows(first, repeat));
    expect(userRows(refresh(settled, first, repeat))).toEqual(expectedRows(first, repeat));
  });

  it("pairs rows written before the cursor existed by message id", () => {
    const legacy = userRow({ content: "legacy", message_id: "legacy-1", round_id: "legacy-round" });

    expect(mergeLoadedMessages([legacy], [legacy])).toHaveLength(1);
    expect(upsertRealtimeMessage([legacy], legacy)).toHaveLength(1);
  });
});

describe("inputs acknowledged without a cursor", () => {
  it("removes the optimistic row of a hidden Room Goal continuation, which has no public user row", () => {
    const earlier = acceptedInput("room", 1);
    const hidden = acceptedInput("room", 2);
    const settled = ack(send([], earlier), earlier);

    const acknowledged = replaceOptimisticUserMessage(send(settled, hidden), {
      client_message_id: hidden.ack.client_message_id,
      round_id: hidden.ack.round_id,
      user_message_committed: false,
      user_message_id: hidden.ack.user_message_id,
    });

    expect(userRows(acknowledged)).toEqual(expectedRows(earlier));
  });
});
