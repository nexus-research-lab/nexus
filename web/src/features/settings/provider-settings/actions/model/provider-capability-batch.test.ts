// INPUT: Deferred model responses, cancellation and transport errors.
// OUTPUT: Three-request bound and no new work after stop/uncertainty.
// POS: Batch scheduler behavior regression.
import { expect, it, vi } from "vitest";
import { runCapabilityBatch } from "./provider-capability-batch";

it("settles in-flight work and never retries after an uncertain failure", async () => {
 const test = vi.fn(async (model:string)=>{if(model==="a")throw new Error("unknown");return model;});
 await expect(runCapabilityBatch({models:["a","b","c","d"],test,shouldStop:()=>false,onProgress:()=>{}})).rejects.toThrow("unknown");
 expect(test).toHaveBeenCalledTimes(3);
});
