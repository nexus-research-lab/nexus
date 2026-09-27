// INPUT: Deferred model responses, cancellation and transport errors.
// OUTPUT: Three-request bound and no new work after stop/uncertainty.
// POS: Batch scheduler behavior regression.
import { expect, it, vi } from "vitest";
import { runCapabilityBatch } from "./provider-capability-batch";

it("starts three models concurrently and fills slots as they finish", async () => {
 const pending = new Map<string, () => void>();
 const test = vi.fn((model: string) => new Promise<string>((resolve) => pending.set(model, () => resolve(model))));
 const onProgress = vi.fn();
 const running = runCapabilityBatch({models:["a","b","c","d"],test,shouldStop:()=>false,onProgress});
 expect(test.mock.calls.map(([id])=>id)).toEqual(["a","b","c"]);
 pending.get("b")!(); await Promise.resolve(); await Promise.resolve();
 expect(test.mock.calls.map(([id])=>id)).toEqual(["a","b","c","d"]);
 for(const id of ["a","c","d"]) pending.get(id)!();
 expect((await running).results.sort()).toEqual(["a","b","c","d"]);
 expect(onProgress).toHaveBeenLastCalledWith(4,4);
});

it("stops queued requests but waits for in-flight results", async () => {
 let stop = false;
 const pending: (()=>void)[]=[];
 const test = vi.fn(()=>new Promise<void>((resolve)=>pending.push(resolve)));
 const running = runCapabilityBatch({models:["a","b","c","d"],test,shouldStop:()=>stop,onProgress:()=>{}});
 stop=true; pending.forEach((resolve)=>resolve());
 expect((await running).stopped).toBe(true);
 expect(test).toHaveBeenCalledTimes(3);
});

it("settles in-flight work and never retries after an uncertain failure", async () => {
 const test = vi.fn(async (model:string)=>{if(model==="a")throw new Error("unknown");return model;});
 await expect(runCapabilityBatch({models:["a","b","c","d"],test,shouldStop:()=>false,onProgress:()=>{}})).rejects.toThrow("unknown");
 expect(test).toHaveBeenCalledTimes(3);
});
