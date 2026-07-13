export type PendingCommandState<Command extends string> = ReadonlyMap<
  Command,
  ReadonlySet<string>
>;

export function createPendingCommandState<Command extends string>(
  commands: readonly Command[],
): PendingCommandState<Command> {
  return new Map(
    commands.map((command) => [command, new Set<string>()]),
  );
}

export function setPendingCommand<Command extends string>(
  current: PendingCommandState<Command>,
  command: Command,
  targetId: string,
  isPending: boolean,
): PendingCommandState<Command> {
  const nextIds = new Set(current.get(command));
  if (isPending) {
    nextIds.add(targetId);
  } else {
    nextIds.delete(targetId);
  }
  return new Map(current).set(command, nextIds);
}
