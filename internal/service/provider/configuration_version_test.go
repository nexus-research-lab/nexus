// INPUT: 并发 Provider patch、模型/默认/测试/发现写入与强制删除故障。
// OUTPUT: 单调 configuration_version、稳定 CAS 冲突、精确不存在错误与事务回滚证明。
// POS: Provider 聚合对话配置写入的一致性回归测试。
package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestProviderPatchAtVersionSerializesConcurrentWriters(t *testing.T) {
	ctx := context.Background()
	service, db := newTestService(t)
	db.SetMaxOpenConns(1)
	created := createConfigurationVersionTestProvider(t, service, ctx, "provider-cas", "https://initial.example.com")
	if created.ConfigurationVersion != 1 {
		t.Fatalf("initial configuration_version = %d, want 1", created.ConfigurationVersion)
	}

	names := []string{"Writer A", "Writer B"}
	results := make([]*Record, len(names))
	errs := make([]error, len(names))
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range names {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errs[index] = service.PatchAtVersion(
				ctx,
				created.Provider,
				PatchInput{DisplayName: &names[index]},
				created.ConfigurationVersion,
			)
		}(index)
	}
	close(start)
	wait.Wait()

	successCount := 0
	conflictCount := 0
	for index, err := range errs {
		switch {
		case err == nil:
			successCount++
			if results[index] == nil || results[index].ConfigurationVersion != 2 {
				t.Fatalf("writer %d result = %+v", index, results[index])
			}
		case errors.Is(err, ErrConfigurationVersionConflict):
			conflictCount++
		default:
			t.Fatalf("writer %d error = %v", index, err)
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("success=%d conflict=%d errors=%v", successCount, conflictCount, errs)
	}
	current, err := service.Get(ctx, created.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if current.ConfigurationVersion != 2 ||
		(current.DisplayName != names[0] && current.DisplayName != names[1]) {
		t.Fatalf("current provider = %+v", current)
	}
}

func TestProviderPatchAtVersionMergesLatestUnmaskedState(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	created := createConfigurationVersionTestProvider(t, service, ctx, "provider-merge", "https://initial.example.com")
	rotatedToken := "rotated-provider-token"
	rotated, err := service.Update(ctx, created.Provider, UpdateInput{
		ProviderKind: created.ProviderKind,
		PresetKey:    created.PresetKey,
		APIFormat:    created.APIFormat,
		DisplayName:  created.DisplayName,
		AuthToken:    &rotatedToken,
		BaseURL:      "https://rotated.example.com",
		ModelsPath:   created.ModelsPath,
		Enabled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.ConfigurationVersion != created.ConfigurationVersion+1 {
		t.Fatalf("rotated configuration_version = %d", rotated.ConfigurationVersion)
	}

	staleName := "Stale Writer"
	if _, err = service.PatchAtVersion(
		ctx,
		created.Provider,
		PatchInput{DisplayName: &staleName},
		created.ConfigurationVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale patch error = %v, want ErrConfigurationVersionConflict", err)
	}

	freshName := "Fresh Writer"
	updated, err := service.PatchAtVersion(
		ctx,
		created.Provider,
		PatchInput{DisplayName: &freshName},
		rotated.ConfigurationVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ConfigurationVersion != rotated.ConfigurationVersion+1 {
		t.Fatalf("updated configuration_version = %d", updated.ConfigurationVersion)
	}
	entity, err := service.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), created.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if entity == nil || entity.DisplayName != freshName ||
		entity.AuthToken != rotatedToken || entity.BaseURL != "https://rotated.example.com" {
		t.Fatalf("merge patch overwrote latest fields: %+v", entity)
	}
	if _, err = service.DeleteAtVersion(
		ctx,
		created.Provider,
		DeleteInput{},
		rotated.ConfigurationVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale delete error = %v, want ErrConfigurationVersionConflict", err)
	}
	if _, err = service.Get(ctx, created.Provider); err != nil {
		t.Fatalf("stale delete removed Provider: %v", err)
	}
}

func TestProviderModelAndDefaultWritesShareAggregateVersion(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	first := createConfigurationVersionTestProvider(t, service, ctx, "provider-first", "https://first.example.com")
	second := createConfigurationVersionTestProvider(t, service, ctx, "provider-second", "https://second.example.com")

	if _, err := service.UpdateModelAtVersion(
		ctx,
		first.Provider,
		"first-model",
		UpdateModelInput{Enabled: true, IsDefault: true},
		first.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	firstAfterDefault, err := service.Get(ctx, first.Provider)
	if err != nil {
		t.Fatal(err)
	}
	secondUnchanged, err := service.Get(ctx, second.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if firstAfterDefault.ConfigurationVersion != 2 || secondUnchanged.ConfigurationVersion != 1 {
		t.Fatalf("versions after first default: first=%d second=%d",
			firstAfterDefault.ConfigurationVersion, secondUnchanged.ConfigurationVersion)
	}

	if _, err = service.UpdateModelAtVersion(
		ctx,
		second.Provider,
		"second-model",
		UpdateModelInput{Enabled: true},
		secondUnchanged.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	secondWithModel, err := service.Get(ctx, second.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetDefaultModelAtVersion(
		ctx,
		second.Provider,
		"second-model",
		secondWithModel.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	firstAfterSwitch, err := service.Get(ctx, first.Provider)
	if err != nil {
		t.Fatal(err)
	}
	secondAfterSwitch, err := service.Get(ctx, second.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if firstAfterSwitch.ConfigurationVersion != 3 || secondAfterSwitch.ConfigurationVersion != 3 {
		t.Fatalf("versions after default switch: first=%d second=%d",
			firstAfterSwitch.ConfigurationVersion, secondAfterSwitch.ConfigurationVersion)
	}
	if len(firstAfterSwitch.Models) != 1 || firstAfterSwitch.Models[0].IsDefault ||
		len(secondAfterSwitch.Models) != 1 || !secondAfterSwitch.Models[0].IsDefault {
		t.Fatalf("default switch failed: first=%+v second=%+v",
			firstAfterSwitch.Models, secondAfterSwitch.Models)
	}

	if _, err = service.UpdateModelAtVersion(
		ctx,
		second.Provider,
		"second-model",
		UpdateModelInput{Enabled: true},
		secondWithModel.ConfigurationVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale model update error = %v", err)
	}

	_, err = service.repository.WithProviderMutation(
		ctx,
		first.ID,
		firstAfterSwitch.ConfigurationVersion,
		func(mutation *providerstore.Mutation) error {
			return mutation.UpdateModel(ctx, providerstore.ModelEntity{
				ID: "missing-model-row", ProviderID: first.ID, ModelID: "missing-model",
			})
		},
	)
	if !errors.Is(err, ErrModelNotFound) {
		t.Fatalf("missing model error = %v, want ErrModelNotFound", err)
	}
	firstAfterRollback, err := service.Get(ctx, first.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if firstAfterRollback.ConfigurationVersion != firstAfterSwitch.ConfigurationVersion {
		t.Fatalf("failed model write advanced version: before=%d after=%d",
			firstAfterSwitch.ConfigurationVersion, firstAfterRollback.ConfigurationVersion)
	}
}

func TestProviderFetchModelsAdvancesVersionExactlyOnce(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"remote-model","name":"Remote Model"}]}`))
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	service, _ := newTestService(t)
	created := createConfigurationVersionTestProvider(t, service, ctx, "provider-fetch", server.URL)
	result, err := service.FetchModelsAtVersion(
		ctx,
		created.Provider,
		created.ConfigurationVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 || len(result.Models) != 1 || result.Models[0].ModelID != "remote-model" {
		t.Fatalf("fetch result = %+v", result)
	}
	if result.Models[0].ContextWindow == nil || *result.Models[0].ContextWindow != defaultModelContextWindow ||
		result.Models[0].MaxOutputTokens == nil || *result.Models[0].MaxOutputTokens != defaultModelMaxOutputTokens {
		t.Fatalf("远端模型缺少卡片信息时未应用默认值: %+v", result.Models[0])
	}
	current, err := service.Get(ctx, created.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if current.ConfigurationVersion != created.ConfigurationVersion+1 {
		t.Fatalf("fetch configuration_version = %d", current.ConfigurationVersion)
	}
	if _, err = service.FetchModelsAtVersion(
		ctx,
		created.Provider,
		created.ConfigurationVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale fetch error = %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("remote request count = %d, want 2", requestCount)
	}
}

func TestProviderTestStateAndModelAdvanceVersionExactlyOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	service, _ := newTestService(t)
	created := createConfigurationVersionTestProvider(t, service, ctx, "provider-test", server.URL)
	result, err := service.TestModelAtVersion(
		ctx,
		created.Provider,
		"tested-model",
		created.ConfigurationVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Status != TestStatusSuccess {
		t.Fatalf("test result = %+v", result)
	}
	current, err := service.Get(ctx, created.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if current.ConfigurationVersion != created.ConfigurationVersion+1 ||
		current.LastTestStatus != TestStatusSuccess ||
		len(current.Models) != 1 || !current.Models[0].Enabled {
		t.Fatalf("provider after test = %+v", current)
	}
	if _, err = service.TestModelAtVersion(
		ctx,
		created.Provider,
		"tested-model",
		created.ConfigurationVersion,
	); !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("stale test error = %v", err)
	}
}

func TestForceDeleteProviderRollsBackRuntimeReassignmentOnDeleteFailure(t *testing.T) {
	ctx := context.Background()
	service, db := newTestService(t)
	fallback := createConfigurationVersionTestProvider(t, service, ctx, "provider-rollback-fallback", "https://fallback.example.com")
	if _, err := service.UpdateModelAtVersion(
		ctx,
		fallback.Provider,
		"fallback-model",
		UpdateModelInput{Enabled: true, IsDefault: true},
		fallback.ConfigurationVersion,
	); err != nil {
		t.Fatal(err)
	}
	target := createConfigurationVersionTestProvider(t, service, ctx, "provider-rollback-target", "https://target.example.com")
	insertProviderUsageAgent(
		t,
		db,
		"agent-provider-rollback",
		"provider-rollback",
		"Provider Rollback",
		"",
		false,
		target.Provider,
		"active",
	)
	if _, err := db.Exec(`
CREATE TRIGGER reject_provider_delete
BEFORE DELETE ON provider
BEGIN
    SELECT RAISE(ABORT, 'forced provider delete failure');
END`); err != nil {
		t.Fatal(err)
	}

	if _, err := service.DeleteAtVersion(
		ctx,
		target.Provider,
		DeleteInput{Force: true},
		target.ConfigurationVersion,
	); err == nil {
		t.Fatal("force delete should fail through trigger")
	}
	if _, err := service.Get(ctx, target.Provider); err != nil {
		t.Fatalf("target provider was not rolled back: %v", err)
	}
	runtime := runtimeSelectionsByAgent(t, db, "agent-provider-rollback")["agent-provider-rollback"]
	if runtime.provider != target.Provider || runtime.version != 1 {
		t.Fatalf("runtime reassignment was not rolled back: %+v", runtime)
	}
}

func createConfigurationVersionTestProvider(
	t *testing.T,
	service *Service,
	ctx context.Context,
	provider string,
	baseURL string,
) *Record {
	t.Helper()
	record, err := service.Create(ctx, CreateInput{
		Provider:    provider,
		PresetKey:   presetCustom,
		APIFormat:   APIFormatResponses,
		DisplayName: provider,
		AuthToken:   "provider-test-token",
		BaseURL:     baseURL,
		ModelsPath:  "/models",
		Enabled:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}
