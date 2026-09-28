// INPUT: Explicit catalog synchronization.
// OUTPUT: Sync returns without launching paid/slow capability probes.
// POS: Discovery and live verification remain separate user actions.
import { act, renderHook, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { ProviderConfigRecord } from "@/types/capability/provider";
import { useProviderModelSync } from "./use-provider-model-sync";

it("syncs only the catalog and refreshes the workspace", async () => {
 const record = { provider: "provider" } as ProviderConfigRecord;
 const fetchModels = vi.fn().mockResolvedValue({ count: 2, models: [] });
 const refreshAll = vi.fn().mockResolvedValue(true);
 const setFeedback = vi.fn();
 const {result} = renderHook(() => useProviderModelSync({modelApi:{fetchModels},persistProvider:vi.fn().mockResolvedValue({record}),refreshAll,runCommand:async(_action,command)=>command(),selectedCanManage:true,selectedRecord:record,setFeedback,t:(key)=>key}));
 act(()=>result.current.handleFetchModels());
 await waitFor(()=>expect(setFeedback).toHaveBeenLastCalledWith(expect.objectContaining({tone:"success"})));
 expect(fetchModels).toHaveBeenCalledExactlyOnceWith("provider");
 expect(refreshAll).toHaveBeenCalledExactlyOnceWith("provider");
});
