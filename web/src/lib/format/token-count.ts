export function formatTokens(tokens: number): string {
  if (tokens < 1_000) {
    return tokens.toString();
  }
  if (tokens < 1_000_000) {
    return `${(tokens / 1_000).toFixed(1)}K`;
  }
  return `${(tokens / 1_000_000).toFixed(1)}M`;
}
