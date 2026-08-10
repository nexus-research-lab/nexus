// INPUT: owner-main 的 Agent 删除计划，以及指向该 Agent 的 Channel/account/pairing/runtime。
// OUTPUT: 单事务级联、Channel version 推进、runtime 注销和目录失效通知。
// POS: configuration Agent 删除跨 agent/storage/channels/app 装配的回归证明。
package configuration_test

import (
	"encoding/json"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/service/channels"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func TestAgentDeleteRevokesChannelStateRuntimeAndNotifies(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Channel Delete Worker")

	if _, err := fixture.services.ChannelControl.UpsertChannelConfig(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.ChannelTypeTelegram,
		channels.UpsertChannelConfigRequest{
			AgentID:     worker.AgentID,
			Credentials: map[string]string{"bot_token": "telegram-token"},
		},
	); err != nil {
		t.Fatal(err)
	}
	pairing, err := fixture.services.ChannelControl.CreatePairing(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.CreatePairingRequest{
			ChannelType: channels.ChannelTypeTelegram,
			ChatType:    "dm",
			ExternalRef: "chat-delete",
			AgentID:     worker.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.DB.Exec(`
INSERT INTO im_channel_accounts (
    owner_user_id, channel_type, account_id, status, config_json
) VALUES (?, 'telegram', 'account-delete', 'connected', '{}')`,
		worker.OwnerUserID,
	); err != nil {
		t.Fatal(err)
	}
	versionBefore, err := fixture.services.ChannelControl.GetChannelControlVersion(
		fixture.ownerCtx,
		worker.OwnerUserID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if fixture.services.Channels.GetForOwner(worker.OwnerUserID, channels.ChannelTypeTelegram) == nil {
		t.Fatal("测试前应存在指向待删除 Agent 的 Channel runtime")
	}

	notifier := &recordingConfigurationNotifier{}
	fixture.services.Configuration.SetNotifier(notifier)
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		IsMainAgent: true,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:agent-channel-delete",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "delete", Target: worker.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID:        "agent-channel-delete-01",
		Domain:           configurationsvc.DomainAgents,
		Operation:        "delete",
		Target:           worker.AgentID,
		Input:            json.RawMessage(`{}`),
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	approveConfigurationTestChange(t, fixture.services, fixture.ownerCtx, actor, request, plan)
	result, err := fixture.services.Configuration.ApplyChange(fixture.ownerCtx, actor, request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !notifier.hasAgent(worker.AgentID) {
		t.Fatalf("Agent 删除应成功并通知目录失效: result=%+v notified=%v", result, notifier.hasAgent(worker.AgentID))
	}
	if fixture.services.Channels.GetForOwner(worker.OwnerUserID, channels.ChannelTypeTelegram) != nil {
		t.Fatal("Agent 删除后 Channel runtime 仍存在")
	}
	if exists, err := fixture.services.ChannelControl.HasChannelConfig(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.ChannelTypeTelegram,
	); err != nil || exists {
		t.Fatalf("Agent 删除后 Channel config 残留: exists=%v err=%v", exists, err)
	}
	if exists, err := fixture.services.ChannelControl.HasChannelAccount(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.ChannelTypeTelegram,
		"account-delete",
	); err != nil || exists {
		t.Fatalf("Agent 删除后 Channel account 残留: exists=%v err=%v", exists, err)
	}
	if exists, err := fixture.services.ChannelControl.HasPairing(
		fixture.ownerCtx,
		worker.OwnerUserID,
		pairing.PairingID,
	); err != nil || exists {
		t.Fatalf("Agent 删除后 pairing 残留: exists=%v err=%v", exists, err)
	}
	versionAfter, err := fixture.services.ChannelControl.GetChannelControlVersion(
		fixture.ownerCtx,
		worker.OwnerUserID,
	)
	if err != nil || versionAfter != versionBefore+1 {
		t.Fatalf("Agent 删除应只推进一次 Channel version: before=%d after=%d err=%v", versionBefore, versionAfter, err)
	}
}

func TestChannelSecretOnlyRotationInvalidatesConversationalPlan(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Channel Version Worker")
	if _, err := fixture.services.ChannelControl.UpsertChannelConfig(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.ChannelTypeTelegram,
		channels.UpsertChannelConfigRequest{
			AgentID:     worker.AgentID,
			Credentials: map[string]string{"bot_token": "initial-token"},
		},
	); err != nil {
		t.Fatal(err)
	}

	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		IsMainAgent: true,
		SessionKey:  "agent:" + fixture.main.AgentID + ":dm:channel-version",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	input, err := json.Marshal(map[string]any{
		"agent_id": worker.AgentID,
		"credentials": map[string]any{
			"bot_token": map[string]string{"$secret": "channel.bot_token"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	stalePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainChannels, Operation: "upsert",
			Target: channels.ChannelTypeTelegram, Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = fixture.services.ChannelControl.UpsertChannelConfig(
		fixture.ownerCtx,
		worker.OwnerUserID,
		channels.ChannelTypeTelegram,
		channels.UpsertChannelConfigRequest{
			AgentID:     worker.AgentID,
			Credentials: map[string]string{"bot_token": "newer-http-token"},
		},
	); err != nil {
		t.Fatal(err)
	}
	currentPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainChannels, Operation: "upsert",
			Target: channels.ChannelTypeTelegram, Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if currentPlan.StateVersion != stalePlan.StateVersion+1 ||
		currentPlan.CurrentRevision == stalePlan.CurrentRevision {
		t.Fatalf("secret-only 轮换必须推进 version/revision: stale=%+v current=%+v", stalePlan, currentPlan)
	}

	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			RequestID: "channel-stale-secret-plan-01",
			Domain:    configurationsvc.DomainChannels, Operation: "upsert",
			Target: channels.ChannelTypeTelegram, Input: input,
			ExpectedRevision: stalePlan.CurrentRevision,
			PlanDigest:       stalePlan.PlanDigest,
		},
	); err == nil {
		t.Fatal("旧 Channel plan 不得覆盖后续 HTTP secret rotation")
	}
	version, err := fixture.services.ChannelControl.GetChannelControlVersion(
		fixture.ownerCtx,
		worker.OwnerUserID,
	)
	if err != nil || version != currentPlan.StateVersion {
		t.Fatalf("失败旧 plan 不得推进 version: version=%d current=%d err=%v", version, currentPlan.StateVersion, err)
	}
}
