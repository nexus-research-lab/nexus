// INPUT: 已规范化的 domain/operation/target/input。
// OUTPUT: 严格字段、必填字段与目标预检结果。
// POS: configuration 写入前的纯校验阶段。
package configuration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	connectorsvc "github.com/nexus-research-lab/nexus/internal/service/connectors"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func requireInputFields(input json.RawMessage, required []string) error {
	if len(required) == 0 {
		return nil
	}
	payload := input
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	var values map[string]any
	if err := json.Unmarshal(payload, &values); err != nil {
		return fmt.Errorf("input 无效: %w", err)
	}
	missing := make([]string, 0)
	for _, field := range required {
		if _, ok := values[field]; !ok {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("input 缺少必填字段: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateChangeRequest(request ChangeRequest) error {
	target := strings.TrimSpace(request.Target)
	requireTarget := func() error {
		if target == "" {
			return fmt.Errorf("%s.%s 要求 target", request.Domain, request.Operation)
		}
		return nil
	}
	decode := func(destination any) error {
		if len(request.Input) == 0 {
			return strictDecodeJSON([]byte(`{}`), destination)
		}
		if !json.Valid(request.Input) {
			return errors.New("input 必须是 JSON object")
		}
		if err := strictDecodeJSON(request.Input, destination); err != nil {
			return fmt.Errorf("input 无效: %w", err)
		}
		return nil
	}
	switch request.Domain + "." + request.Operation {
	case DomainPreferences + ".update":
		return decode(&preferencessvc.UpdateRequest{})
	case DomainProviders + ".create":
		return decode(&providerCreateRequest{})
	case DomainProviders + ".update":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&providerUpdateRequest{})
	case DomainProviders + ".delete":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&providersvc.DeleteInput{})
	case DomainProviders + ".fetch_models", DomainProviders + ".test_provider":
		return requireTarget()
	case DomainProviders + ".update_model":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&providerModelMutation{})
	case DomainProviders + ".set_default_model", DomainProviders + ".test_model":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&providerModelTarget{})
	case DomainAgents + ".create":
		return decode(&protocol.CreateRequest{})
	case DomainAgents + ".update":
		if err := requireTarget(); err != nil {
			return err
		}
		var input agentUpdatePatch
		if err := decode(&input); err != nil {
			return err
		}
		if len(input.Options) > 0 && string(input.Options) != "null" {
			return strictDecodeJSON(input.Options, &protocol.Options{})
		}
		return nil
	case DomainAgents + ".delete":
		return requireTarget()
	case DomainChannels + ".upsert":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&channels.UpsertChannelConfigRequest{})
	case DomainChannels + ".delete_config":
		return requireTarget()
	case DomainChannels + ".delete_account":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&channelAccountTarget{})
	case DomainChannels + ".create_pairing":
		return decode(&channels.CreatePairingRequest{})
	case DomainChannels + ".update_pairing":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&channels.UpdatePairingRequest{})
	case DomainChannels + ".delete_pairing":
		return requireTarget()
	case DomainConnectors + ".connect":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&connectorCredentials{})
	case DomainConnectors + ".disconnect", DomainConnectors + ".delete_oauth_client":
		return requireTarget()
	case DomainConnectors + ".save_oauth_client":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&connectorsvc.OAuthClientConfigRequest{})
	case DomainSkills + ".update_source":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&skillsvc.ExternalSkillSourceRequest{})
	case DomainSkills + ".install", DomainSkills + ".uninstall":
		if err := requireTarget(); err != nil {
			return err
		}
		return decode(&skillAgentTarget{})
	case DomainSkills + ".delete":
		return requireTarget()
	case DomainSkills + ".check_updates", DomainSkills + ".update_imported":
		return nil
	default:
		return fmt.Errorf("不支持配置操作 %s.%s", request.Domain, request.Operation)
	}
}

func strictDecodeJSON(payload []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("input 只能包含一个 JSON object")
		}
		return err
	}
	return nil
}
