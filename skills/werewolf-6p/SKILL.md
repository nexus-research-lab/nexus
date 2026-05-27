---
name: werewolf-6p
title: Six-Player Werewolf Debug Script
description: Room communication validation rules for one host member and six player members.
scope: room
tags: [room, game, werewolf]
---

# Six-Player Werewolf Debug Script

This script is only for validating Room communication mechanics. It does not move game state into platform-owned logic. The host member maintains the game state. Room only handles public feed, private delivery, audience night chat, request replies, and context projection.

## Thinking Budget

- The host member should focus only on the current phase, actions to send, and who must reply next. Do not restate the full rules, full role table, or fallback plans in reasoning.
- Player members speak only from visible information. Do not write long recaps of hidden information.
- Keep each public statement under 120 words, each private request under 160 words, and each private note under 12 lines.
- Advance one closed step at a time. Do not create actions for multiple future phases at once.

## Players And Roles

- 1 host member: assigns roles, collects night actions, announces daybreak, organizes speeches, and runs voting.
- 6 player members: 2 werewolves, 1 seer, 1 witch, and 2 villagers.
- Roles should be privately randomized by the host or assigned as needed for testing, then sent to each player with `private_message --wake-policy none`.
- The host uses `private_note` to maintain minimal state: round, alive players, dead players, roles, witch potions, and currently awaited items.

## Win Conditions

- Good side wins when both werewolves are eliminated.
- Werewolves win when all villagers die or all special roles die.
- Check win conditions after each daybreak announcement and each voted elimination. Continue to the next phase only if the game has not ended.

## Night Flow

Close these steps in order:

1. Werewolf action: send `private_message --audience-agent-id ... --wake-policy immediate` to the two werewolves to start night chat. Then send `request-reply --reply-target sender_private` to one werewolf submitter and ask for only the kill target name.
2. Seer action: after receiving the werewolf kill target, send `request-reply --reply-target sender_private` to the seer and ask for only the inspection target name. Then use `private_message --wake-policy none` to return either "good side" or "werewolf".
3. Witch action: send `request-reply --reply-target sender_private` to the witch, include tonight's killed player and remaining potions, and ask for only "save/no save; poison/no poison player name".
4. Daybreak: resolve deaths from kill, antidote, and poison. Announce only the death result in the public feed. Do not reveal roles or night private-chat content.

## Day Flow

- The host wakes players one by one in a fixed order with `request-reply --reply-target public_feed`.
- After all living players have spoken, the host collects votes one by one, or appoints one host-side collector during debugging.
- Vote results announce only the eliminated player and vote shape. Do not reveal undisclosed roles.

## Room Action Constraints

- Werewolf night chat uses audience private context. Discussion between werewolves is projected only to the two werewolves.
- When someone must submit a decision to the host, use `request_reply`; do not make players call the CLI.
- When a player receives `request_reply`, the final reply is the answer. Do not create a new Room action unless the request explicitly asks for forwarding to a third party.
- After the host receives a private reply, update private state and start the next step. Public output is only for phase announcements, death announcements, speeches, and vote results.
