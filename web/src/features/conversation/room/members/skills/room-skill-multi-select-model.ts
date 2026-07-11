export interface RoomSkillOption {
  description: string;
  label: string;
  value: string;
}

export type RoomSkillMenuBodyKind = "empty" | "error" | "loading" | "options";

export interface RoomSkillMenuBodyPresentation {
  kind: RoomSkillMenuBodyKind;
  message: string;
  options: RoomSkillOption[];
}

interface RoomSkillMenuBodyInput {
  emptyText: string;
  errorText: string | null;
  isLoading: boolean;
  loadingText: string;
  options: RoomSkillOption[];
}

interface RoomSkillMenuBodyRule {
  build: (input: RoomSkillMenuBodyInput) => RoomSkillMenuBodyPresentation;
  matches: (input: RoomSkillMenuBodyInput) => boolean;
}

const MENU_BODY_RULES: RoomSkillMenuBodyRule[] = [
  {
    build: ({ loadingText }) => ({
      kind: "loading",
      message: loadingText,
      options: [],
    }),
    matches: ({ isLoading }) => isLoading,
  },
  {
    build: ({ errorText }) => ({
      kind: "error",
      message: errorText || "",
      options: [],
    }),
    matches: ({ errorText }) => Boolean(errorText),
  },
  {
    build: ({ emptyText }) => ({
      kind: "empty",
      message: emptyText,
      options: [],
    }),
    matches: ({ options }) => options.length === 0,
  },
];

const OPTIONS_BODY_RULE: RoomSkillMenuBodyRule = {
  build: ({ options }) => ({ kind: "options", message: "", options }),
  matches: () => true,
};

export function buildRoomSkillMenuBody(
  input: RoomSkillMenuBodyInput,
): RoomSkillMenuBodyPresentation {
  const rule = MENU_BODY_RULES.find((candidate) => candidate.matches(input))
    ?? OPTIONS_BODY_RULE;
  return rule.build(input);
}

export function buildSelectedRoomSkills(
  options: RoomSkillOption[],
  values: string[],
): RoomSkillOption[] {
  const optionsByValue = new Map(
    options.map((option) => [option.value, option]),
  );
  return values.map((value) => optionsByValue.get(value) ?? {
    description: "",
    label: value,
    value,
  });
}

export function toggleRoomSkill(
  values: string[],
  value: string,
): string[] {
  if (values.includes(value)) {
    return values.filter((item) => item !== value);
  }
  return [...values, value];
}

export function removeRoomSkill(
  values: string[],
  value: string,
): string[] {
  return values.filter((item) => item !== value);
}
