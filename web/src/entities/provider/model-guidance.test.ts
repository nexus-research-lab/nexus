import { expect, it } from "vitest";
import { modelGuidanceLabel, recommendedModelsFirst } from "./model-guidance";
import type { ModelGuidance } from "@/types/capability/provider";
const guidance: ModelGuidance = {
 catalog_version: "test", capabilities: { vision: true, image_output: true }, sources: {},
 eligibility: {chat:{available:true}, vision:{available:true}, image_generation:{available:true}, image_editing:{available:false}},
 recommendations: { image_generation: "image_generation" },
};
it("uses purpose-specific advice independently of capability badges", () => {
 const t = (key: string) => key;
 expect(modelGuidanceLabel(guidance,"chat",t)).not.toContain("model_recommended");
 expect(modelGuidanceLabel(guidance,"image_generation",t)).toContain("model_recommended");
 expect(modelGuidanceLabel(guidance,"chat",t)).toContain("model_multimodal");
 expect(modelGuidanceLabel(undefined,"chat",t)).toBe("");
});
it("sorts recommendations without mutating the original or saved default", () => {
 const models=[{id:"custom",is_default:true,guidance:undefined},{id:"image",is_default:false,guidance}];
 expect(recommendedModelsFirst(models,"image_generation").map(m=>m.id)).toEqual(["image","custom"]);
 expect(models[0].id).toBe("custom");
 expect(models[0].is_default).toBe(true);
 expect(recommendedModelsFirst(models,"chat")).toEqual(models);
});
