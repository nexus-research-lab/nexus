// INPUT: ScheduledTask 的执行/投递会话目标与已删除会话键。
// OUTPUT: 可持久恢复的会话绑定健康状态、受影响范围与重绑后的自动归一化结果。
// POS: Automation 任务会话绑定生命周期的领域真相源；来源 Session 只作 provenance，不参与失效判定。
package types

import (
	"errors"
	"sort"
	"strings"
)

const (
	// TaskSessionBindingStateReady 表示任务没有引用已删除的 Session。
	TaskSessionBindingStateReady = "ready"
	// TaskSessionBindingStateRebindRequired 表示任务已暂停，等待替换所有失效 Session。
	TaskSessionBindingStateRebindRequired = "rebind_required"

	// TaskSessionBindingIssueExecution 表示执行上下文引用了已删除 Session。
	TaskSessionBindingIssueExecution = "execution"
	// TaskSessionBindingIssueDelivery 表示结果投递引用了已删除 Session。
	TaskSessionBindingIssueDelivery = "delivery"
)

// ErrTaskSessionRebindRequired 阻止带失效会话绑定的任务启用或执行。
var ErrTaskSessionRebindRequired = errors.New(
	"scheduled task has deleted session bindings; reassign every affected execution or delivery session before enabling",
)

// ErrTaskDeliverySessionUnavailable 表示用户选择的外部 IM 目标没有当前 active pairing。
var ErrTaskDeliverySessionUnavailable = errors.New(
	"目标 IM 会话当前未配对，不能作为定时任务投递目标；请重新连接通道后选择当前会话",
)

// NormalizeScheduledTaskSessionBinding 清理已被用户替换的失效键并重建展示状态。
func NormalizeScheduledTaskSessionBinding(task ScheduledTask) ScheduledTask {
	invalidated := normalizeSessionKeys(task.InvalidatedSessionKeys)
	remaining := make([]string, 0, len(invalidated))
	issueSet := make(map[string]struct{}, 2)
	for _, sessionKey := range invalidated {
		issues := scheduledTaskSessionBindingIssuesForKey(task, sessionKey)
		if len(issues) == 0 {
			continue
		}
		remaining = append(remaining, sessionKey)
		for _, issue := range issues {
			issueSet[issue] = struct{}{}
		}
	}
	task.InvalidatedSessionKeys = remaining
	if len(remaining) == 0 {
		task.SessionBindingState = TaskSessionBindingStateReady
		task.SessionBindingIssues = nil
		return task
	}
	task.SessionBindingState = TaskSessionBindingStateRebindRequired
	task.SessionBindingIssues = orderedSessionBindingIssues(issueSet)
	return task
}

// InvalidateScheduledTaskSessions 标记任务当前引用的已删除 Session，并强制停用。
func InvalidateScheduledTaskSessions(
	task ScheduledTask,
	deletedSessionKeys []string,
) (ScheduledTask, bool) {
	before := NormalizeScheduledTaskSessionBinding(task)
	invalidated := append([]string(nil), before.InvalidatedSessionKeys...)
	for _, sessionKey := range normalizeSessionKeys(deletedSessionKeys) {
		if len(scheduledTaskSessionBindingIssuesForKey(before, sessionKey)) > 0 {
			invalidated = append(invalidated, sessionKey)
		}
	}
	task.InvalidatedSessionKeys = invalidated
	task = NormalizeScheduledTaskSessionBinding(task)
	if task.SessionBindingState == TaskSessionBindingStateRebindRequired {
		task.Enabled = false
	}
	changed := before.Enabled != task.Enabled ||
		before.SessionBindingState != task.SessionBindingState ||
		!sameStrings(before.InvalidatedSessionKeys, task.InvalidatedSessionKeys) ||
		!sameStrings(before.SessionBindingIssues, task.SessionBindingIssues)
	return task, changed
}

func scheduledTaskSessionBindingIssuesForKey(task ScheduledTask, sessionKey string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	issues := make(map[string]struct{}, 2)
	if strings.TrimSpace(task.SessionTarget.BoundSessionKey) == sessionKey ||
		strings.TrimSpace(task.SessionTarget.NamedSessionKey) == sessionKey {
		issues[TaskSessionBindingIssueExecution] = struct{}{}
	}
	if strings.TrimSpace(task.Delivery.SessionKey) == sessionKey ||
		strings.TrimSpace(task.Delivery.To) == sessionKey {
		issues[TaskSessionBindingIssueDelivery] = struct{}{}
	}
	return orderedSessionBindingIssues(issues)
}

func normalizeSessionKeys(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func orderedSessionBindingIssues(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for _, issue := range []string{
		TaskSessionBindingIssueExecution,
		TaskSessionBindingIssueDelivery,
	} {
		if _, exists := values[issue]; exists {
			result = append(result, issue)
		}
	}
	return result
}

func sameStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
