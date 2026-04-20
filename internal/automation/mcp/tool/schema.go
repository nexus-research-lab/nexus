package tool

var (
	scheduleSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":             map[string]any{"type": "string", "enum": []string{"every", "cron", "at"}},
			"interval_seconds": map[string]any{"type": "integer"},
			"cron_expression":  map[string]any{"type": "string"},
			"run_at":           map[string]any{"type": "string"},
			"timezone":         map[string]any{"type": "string"},
		},
		"required": []string{"kind"},
	}

	sessionTargetSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":              map[string]any{"type": "string", "enum": []string{"isolated", "main", "bound", "named"}},
			"bound_session_key": map[string]any{"type": "string"},
			"named_session_key": map[string]any{"type": "string"},
			"wake_mode":         map[string]any{"type": "string", "enum": []string{"now", "next-heartbeat"}},
		},
		"required": []string{"kind"},
	}

	deliverySchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode":       map[string]any{"type": "string", "enum": []string{"none", "last", "explicit"}},
			"channel":    map[string]any{"type": "string"},
			"to":         map[string]any{"type": "string"},
			"account_id": map[string]any{"type": "string"},
			"thread_id":  map[string]any{"type": "string"},
		},
	}

	sourceSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":             map[string]any{"type": "string", "enum": []string{"user_page", "agent", "cli", "system"}},
			"creator_agent_id": map[string]any{"type": "string"},
			"context_type":     map[string]any{"type": "string", "enum": []string{"agent", "room"}},
			"context_id":       map[string]any{"type": "string"},
			"context_label":    map[string]any{"type": "string"},
			"session_key":      map[string]any{"type": "string"},
			"session_label":    map[string]any{"type": "string"},
		},
	}

	executionModeSchema = map[string]any{
		"type": "string",
		"enum": []string{"main", "existing", "temporary", "dedicated", "current_chat"},
	}

	replyModeSchema = map[string]any{
		"type": "string",
		"enum": []string{"none", "execution", "selected", "current_chat"},
	}
)

func createSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":                       map[string]any{"type": "string"},
			"agent_id":                   map[string]any{"type": "string"},
			"instruction":                map[string]any{"type": "string"},
			"schedule":                   scheduleSchema,
			"execution_mode":             executionModeSchema,
			"reply_mode":                 replyModeSchema,
			"named_session_key":          map[string]any{"type": "string"},
			"selected_session_key":       map[string]any{"type": "string"},
			"selected_reply_session_key": map[string]any{"type": "string"},
			"session_target":             sessionTargetSchema,
			"delivery":                   deliverySchema,
			"source":                     sourceSchema,
			"enabled":                    map[string]any{"type": "boolean"},
		},
		"required": []string{"name", "instruction", "schedule"},
	}
}

func updateSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"job_id":         map[string]any{"type": "string"},
			"name":           map[string]any{"type": "string"},
			"instruction":    map[string]any{"type": "string"},
			"schedule":       scheduleSchema,
			"session_target": sessionTargetSchema,
			"delivery":       deliverySchema,
			"source":         sourceSchema,
			"enabled":        map[string]any{"type": "boolean"},
		},
		"required": []string{"job_id"},
	}
}

func jobIDSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"job_id": map[string]any{"type": "string"}},
		"required":   []string{"job_id"},
	}
}
