import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import type { CommandDescriptor } from "@/types/generated/protocol";
import type { ProviderOptionsResponse } from "@/types/capability/provider";
import type { SkillInfo } from "@/types/capability/skill";

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

export interface SlashModelOption {
  id: string;
  label: string;
  provider?: string;
  providerLabel?: string;
}

type SkillDescriptionResolver = (skill: SkillInfo) => string;

const slashCommandNameCollator = new Intl.Collator("en", {
  numeric: true,
  sensitivity: "base",
});

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
  const search = createUiSearchMatcher(query);
  const matchingCommands = commands.filter((command) => search.matches([
    normalizeSlashCommandName(command.name),
  ], "prefix"));
  return matchingCommands.sort((left, right) => slashCommandNameCollator.compare(
    normalizeSlashCommandName(left.name),
    normalizeSlashCommandName(right.name),
  ));
}

function normalizeSlashCommandName(name: string): string {
  return name.trim().replace(/^\/+/, "");
}

export function filterSlashSkills(
  skills: SkillInfo[],
  query: string,
  resolveDescription: SkillDescriptionResolver = (skill) => skill.description,
): SkillInfo[] {
  const search = createUiSearchMatcher(query);
  return skills.filter((skill) => search.matches([
    skill.name,
    skill.title,
    resolveDescription(skill),
    skill.category_name,
    skill.source_name,
    ...skill.tags,
  ]));
}

export function filterSlashModels(
  models: SlashModelOption[],
  query: string,
): SlashModelOption[] {
  const search = createUiSearchMatcher(query);
  return models.filter((model) => search.matches([
    model.id,
    model.label,
    model.providerLabel,
  ]));
}

export function isSelectableSlashCommand(
  command: CommandDescriptor,
): boolean {
  return command.enabled && (
    command.execution === "host"
    || command.execution === "runtime"
  );
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

export function formatSlashCommandInsertText(name: string): string {
  return `/${name.trim().replace(/^\/+/u, "")} `;
}

export function formatSlashModelInsertText(model: SlashModelOption): string {
  const modelID = model.id.trim();
  const provider = model.provider?.trim();
  return `/model ${provider ? `${provider}/${modelID}` : modelID} `;
}

export function insertSlashTextAtCursor(
  input: string,
  cursorPosition: number,
  commandText: string,
): SlashCommandInsertion {
  const safeCursorPosition = Math.max(
    0,
    Math.min(cursorPosition, input.length),
  );
  return {
    cursorPosition: safeCursorPosition + commandText.length,
    value: [
      input.slice(0, safeCursorPosition),
      commandText,
      input.slice(safeCursorPosition),
    ].join(""),
  };
}

export function buildSlashModelOptions(
  response: ProviderOptionsResponse | null,
): SlashModelOption[] {
  const providerOptions = (response?.items ?? []).flatMap((provider) =>
    provider.models.map((model) => ({
      id: model.model_id.trim(),
      label: model.display_name.trim() || model.model_id.trim(),
      provider: provider.provider.trim(),
      providerLabel: provider.display_name.trim() || provider.provider.trim(),
    })),
  );
  return dedupeSlashModelOptions(providerOptions);
}

function dedupeSlashModelOptions(
  options: readonly SlashModelOption[],
): SlashModelOption[] {
  const seen = new Set<string>();
  return options.filter((option) => {
    const id = option.id.trim();
    const provider = option.provider?.trim() ?? "";
    const key = `${provider}\u0000${id}`.toLocaleLowerCase();
    if (!id || seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}
