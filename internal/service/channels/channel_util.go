package channels

import (
	"database/sql"

	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
)

func nullableString(value string) any {
	return channelcontract.NullableString(value)
}

func nullStringValue(value sql.NullString) string {
	return channelcontract.NullStringValue(value)
}

func firstNonEmpty(values ...string) string {
	return channelcontract.FirstNonEmpty(values...)
}

func newDeliveryID(prefix string) string {
	return channelcontract.NewID(prefix)
}
