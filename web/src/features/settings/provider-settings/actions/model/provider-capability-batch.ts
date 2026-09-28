// INPUT: Exact model IDs, an explicit test callback and a stop flag.
// OUTPUT: Settled observations with at most three concurrent model requests.
// POS: Bounded test scheduling; stop/error prevents new requests and never replays work.
export async function runCapabilityBatch<T>(options: {
  models: string[];
  test: (model: string) => Promise<T>;
  shouldStop: () => boolean;
  onProgress: (done: number, total: number) => void;
}): Promise<{ results: T[]; stopped: boolean }> {
  const models = [...new Set(options.models)];
  const results: T[] = [];
  let next = 0;
  let done = 0;
  let failure: unknown;
  let failed = false;
  options.onProgress(done, models.length);
  async function worker() {
    while (!failed && !options.shouldStop() && next < models.length) {
      const model = models[next++];
      try {
        results.push(await options.test(model));
      } catch (error) {
        failed = true;
        failure = error;
      } finally {
        options.onProgress(++done, models.length);
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(3, models.length) }, worker));
  if (failed) throw failure;
  return { results, stopped: next < models.length };
}
