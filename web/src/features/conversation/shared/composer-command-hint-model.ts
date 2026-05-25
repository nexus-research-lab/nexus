export interface ComposerCommandHintItem {
  command: string;
  detail: string;
  insert_text?: string;
}

export function filter_composer_command_hints(
  input: string,
  items: ComposerCommandHintItem[],
): ComposerCommandHintItem[] {
  const query = input.trimStart().toLowerCase();
  if (!query.startsWith("/") || query.includes("\n")) {
    return [];
  }
  return items
    .filter((item) => item.command.toLowerCase().startsWith(query))
    .slice(0, 5);
}
