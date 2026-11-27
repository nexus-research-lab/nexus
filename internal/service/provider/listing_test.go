package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestBatchedModelReadsMatchPerProviderReads(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	providerIDs := make([]string, 0, 2)
	for index, name := range []string{"image-a", "image-b"} {
		record, err := service.Create(ctx, CreateInput{
			ProviderKind: ProviderKindImageGeneration,
			Provider:     name,
			PresetKey:    presetCustom,
			APIFormat:    APIFormatModelScopeImageGeneration,
			AuthToken:    "image-key",
			BaseURL:      "https://api-inference.modelscope.cn/v1",
			Enabled:      true,
			DisplayName:  name,
		})
		if err != nil {
			t.Fatalf("创建 provider 失败: %v", err)
		}
		providerIDs = append(providerIDs, record.ID)
		now := service.now()
		models := make([]providerstore.ModelEntity, 0, 3)
		for _, modelID := range []string{"z-model", "a-model", "m-model"} {
			models = append(models, providerstore.ModelEntity{
				ID:                       service.idFactory("provider_model"),
				ProviderID:               record.ID,
				ModelID:                  fmt.Sprintf("%s-%d", modelID, index),
				DisplayName:              modelID,
				Category:                 "image",
				Enabled:                  modelID != "m-model",
				CapabilitiesAutoJSON:     "{}",
				CapabilitiesOverrideJSON: "{}",
				ProviderOptionsJSON:      "{}",
				LastSeenAt:               now,
				CreatedAt:                now,
				UpdatedAt:                now,
			})
		}
		if err := service.repository.UpsertModels(ctx, models); err != nil {
			t.Fatalf("写入模型失败: %v", err)
		}
	}

	batched, err := service.repository.ListModelsByProviderIDs(ctx, append(providerIDs, providerIDs[0], " "))
	if err != nil {
		t.Fatalf("批量读取失败: %v", err)
	}
	if len(batched) != len(providerIDs) {
		t.Fatalf("批量结果应按 Provider 去重分组: %d", len(batched))
	}
	for _, providerID := range providerIDs {
		single, err := service.repository.ListModelsByProviderID(ctx, providerID)
		if err != nil {
			t.Fatalf("单个读取失败: %v", err)
		}
		// 组内顺序必须与单个读取一致：enabled 优先，再按名称。
		if len(single) != 3 || !reflect.DeepEqual(batched[providerID], single) {
			t.Fatalf("批量与单个读取不一致:\n%+v\n%+v", batched[providerID], single)
		}
	}

	options, err := service.ListOptionsForRuntime(ctx, "nxs")
	if err != nil {
		t.Fatalf("读取 options 失败: %v", err)
	}
	if len(options.ImageItems) != 2 || len(options.ImageItems[0].Models) != 2 || len(options.ImageItems[1].Models) != 2 {
		t.Fatalf("图片选项应只含各自启用的模型: %+v", options.ImageItems)
	}
}
