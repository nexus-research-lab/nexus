// INPUT: 对话友好的 merge-patch 输入与现有 Provider、Agent、Room、Preferences 领域记录。
// OUTPUT: 不重置未声明字段的各领域原生服务输入。
// POS: configuration 对话补丁到既有完整快照 API 的适配层。
package configuration

import (
	"encoding/json"
)

func mergeJSONObject(current any, patch json.RawMessage, excludedKeys ...string) (json.RawMessage, error) {
	currentPayload, err := json.Marshal(current)
	if err != nil {
		return nil, err
	}
	var currentMap map[string]any
	if err = json.Unmarshal(currentPayload, &currentMap); err != nil {
		return nil, err
	}
	var patchMap map[string]any
	if len(patch) == 0 {
		patch = json.RawMessage(`{}`)
	}
	if err = json.Unmarshal(patch, &patchMap); err != nil {
		return nil, err
	}
	for _, key := range excludedKeys {
		delete(patchMap, key)
	}
	mergeMap(currentMap, patchMap)
	return json.Marshal(currentMap)
}

func mergeMap(current map[string]any, patch map[string]any) {
	for key, patchValue := range patch {
		if patchValue == nil {
			delete(current, key)
			continue
		}
		patchMap, patchIsMap := patchValue.(map[string]any)
		currentMap, currentIsMap := current[key].(map[string]any)
		if patchIsMap && currentIsMap {
			mergeMap(currentMap, patchMap)
			current[key] = currentMap
			continue
		}
		current[key] = patchValue
	}
}
