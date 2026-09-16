package communication

import "testing"

func TestIMQueryRejectsScopeWideningBeforeService(t *testing.T) {
	for _, args := range []map[string]any{
		{"scope": "delivery_sources", "session_key": "other"},
		{"scope": "address_book", "delivery_id": "d"},
		{"scope": "delivery_sources", "delivery_id": "d", "query": "other"},
		{"scope": "delivery_sources", "limit": 1.5},
		{"scope": "delivery_sources", "limit": 51},
		{"scope": "delivery_sources", "offset": -1},
		{"scope": true},
		{"scope": ""},
		{"scope": "delivery_sources", "delivery_id": ""},
	} {
		if _, _, err := parseDeliveryQuery(args); err == nil {
			t.Fatalf("accepted %+v", args)
		}
	}
	if _, lookup, err := parseDeliveryQuery(nil); err != nil || lookup {
		t.Fatalf("default directory changed %v %v", lookup, err)
	}
	q, lookup, err := parseDeliveryQuery(map[string]any{"scope": "delivery_sources", "query": "草案", "limit": float64(5)})
	if err != nil || !lookup || q.Limit != 5 || q.Query != "草案" {
		t.Fatalf("query %+v %v", q, err)
	}
}
