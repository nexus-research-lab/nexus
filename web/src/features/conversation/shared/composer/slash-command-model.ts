import type { CommandDescriptor } from "@/types/generated/protocol";

export const SLASH_COMMAND_NAVIGATION_KEYS = new Set([
  "ArrowDown",
  "ArrowUp",
  "Enter",
  "Tab",
  "Escape",
]);

export interface SlashCommandTextMatch {
  end: number;
  query: string;
  start: number;
}

export interface SlashCommandInsertion {
  cursorPosition: number;
  value: string;
}

export function findSlashCommandTextMatch(
  input: string,
  cursorPosition: number,
  enabled: boolean,
): SlashCommandTextMatch | null {
  if (!enabled) {
    return null;
  }
  const safeCursorPosition = Math.max(
    0,
    Math.min(cursorPosition, input.length),
  );
  const match = input.slice(0, safeCursorPosition).match(/^\/([^\s/]*)$/u);
  if (!match) {
    return null;
  }
  return {
    end: safeCursorPosition,
    query: match[1] ?? "",
    start: 0,
  };
}

export function filterSlashCommands(
  commands: CommandDescriptor[],
  query: string,
): CommandDescriptor[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  if (!normalizedQuery) {
    return commands;
  }
  return commands.filter((command) => {
    const searchableText = [
      command.name,
      command.description ?? "",
      command.argument_hint ?? "",
    ].join("\n").toLocaleLowerCase();
    return searchableText.includes(normalizedQuery);
  });
}

export function isSelectableSlashCommand(
  command: CommandDescriptor,
): boolean {
  return command.enabled && command.execution === "runtime_prompt";
}

export function insertSlashCommand(
  input: string,
  match: SlashCommandTextMatch,
  command: CommandDescriptor,
): SlashCommandInsertion {
  const commandText = `/${command.name.trim().replace(/^\/+/u, "")} `;
  return {
    cursorPosition: match.start + commandText.length,
    value: [
      input.slice(0, match.start),
      commandText,
      input.slice(match.end),
    ].join(""),
  };
}
