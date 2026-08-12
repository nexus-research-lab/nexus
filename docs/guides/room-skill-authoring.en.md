# Room Skill Authoring Guide

## 1. Purpose

This guide is for Skill authors. It explains how to express business collaboration rules as an executable Room Skill.

Communication fields, visibility, and wake-up constraints are defined by the [Room collaboration protocol](../specs/room-collaboration-spec.md). This guide does not duplicate those implementation details.

## 2. When to use a Room Skill

Use a Room Skill for work that requires multiple agents in the same conversation, such as:

- Explicit host, execution, review, or synthesis roles.
- Private collection followed by a public conclusion.
- Agreed speaking order, handoff, silence, or completion conditions.

Rules that affect only one agent belong in a regular Agent Skill.

## 3. What a Room Skill must define

### 3.1 Activation conditions

State the applicable tasks, triggers, and exclusions. Do not assume every Room follows the same business workflow.

### 3.2 Member responsibilities

Give each member role an observable responsibility: who initiates, executes, reviews, synthesizes, and publishes the final conclusion.

### 3.3 Public collaboration

Publish only facts every member needs in the public area:

- Goals and confirmed decisions.
- One-off conversation requests that do not require durable ownership or acceptance.
- Milestone progress, blockers, and final conclusions.

Do not publish unconfirmed drafts, repeated acknowledgements, or empty agreement. Use a real, non-code `@member` only when action is required. Use an ordinary name when describing a plan, example, or status so the mention does not wake the agent.

`@member` is conversation wake/handoff only. It creates no Work Item, Assignment, Submission, or Acceptance, and it does not define speaking order. Durable ownership, dependencies, delivery, and acceptance require a WorkGraph whose responsibility is established by `assign_work`. When agents work in parallel, the Skill must not assume which one finishes first. Nexus dispatches each source slot independently and shows public messages in their actual publish order.

### 3.4 Private collaboration

Use private messages for:

- Requests only a specific member can handle.
- Background or intermediate opinions that should not be public.
- Information that must be collected privately and synthesized by a designated member.

Private information is not published automatically. When publishing a conclusion, repeat only what is necessary and do not copy sensitive source text.

### 3.5 Handoff and completion

The Skill must state:

- When work is handed to the next member.
- Who receives the synthesized result.
- Who publishes the final conclusion.
- How to stop when no new work remains.
- Who intervenes after a timeout, failure, or unavailable member.

Nexus does not decide when the discussion is complete and does not automatically return control to the host.

## 4. Choosing tools and reply routes

| Scenario | Action |
| --- | --- |
| Ordinary public reply | Return the final reply directly; do not call a Room tool. |
| One-off public conversation request or wake | Include a real, non-code `@member` in the final reply; it does not establish tracked responsibility. |
| Accountable delivery, dependency, or acceptance | Use WorkGraph and `assign_work`; return through `submit_work` / `review_work` instead of using `@` as a substitute. |
| Private send or multi-member collection | Use `send_directed_message` and specify recipients, wake behavior, and `reply_route`. |
| Return a private result to the host | Use `reply_route=private`; set `wake_policy=immediate` when needed. |
| Let the host continue publicly from a private result | Set `next_reply_route=public` on the private route. |
| Broadcast a separate fact from a private or tool-driven flow | Use `publish_public_message` once; ordinary public replies should not use it. |
| Woken with no new work | Return `<nexus_room_no_reply/>` instead of creating a public message. |

The runtime injects Room, conversation, source-agent, and causal fields for tools. A Skill must not forge or assemble them.

## 5. Loading a Room Skill

Room Skills use the ordinary Skill directory, frontmatter, and body format. The Room runtime loads the Skill directly when starting a member. It injects the complete body after removing frontmatter, so the body is the single source of truth for Room collaboration rules.

`description` is only for the Skill catalog and management UI. Do not maintain a second runtime-rules field.

## 6. Pre-release checklist

- Does the Skill declare `scope: room`?
- Does it define member responsibilities and the final synthesizer?
- Does it distinguish public facts from private context?
- Does it mention a member only when action is required?
- Do directed messages define `recipients`, `wake_policy`, and `reply_route`?
- Does it explain how private results return to the public area?
- Does it define stopping, timeout, and failure behavior?
- Does it avoid workflow states that Nexus cannot verify?

In short: the Skill defines why members collaborate, who does what, and when work ends; Nexus delivers messages, schedules execution, and routes replies.
