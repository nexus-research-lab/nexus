package echo

import "testing"

func TestDefaultPolicyIsInternalAndDisabled(t *testing.T) {
	t.Parallel()
	policy := DefaultPolicy("Asia/Shanghai")
	if policy.Timezone != "Asia/Shanghai" || policy.Enabled {
		t.Fatalf("DefaultPolicy() = %+v, want disabled user policy", policy)
	}
}
